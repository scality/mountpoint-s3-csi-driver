/*
Copyright 2022 The Kubernetes Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package node

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	// AgentNotReadyNodeTaintKey is the taint key applied to nodes to prevent
	// workload scheduling before the CSI driver has registered with kubelet.
	// Cluster admins should pre-taint nodes with this key using NoExecute effect.
	// The CSI DaemonSet tolerates this taint and removes it once the driver
	// is registered, preventing "driver not found" errors during node startup.
	AgentNotReadyNodeTaintKey = "s3.csi.scality.com/agent-not-ready"

	// TaintWatcherDuration is the maximum time the taint watcher will run
	// before giving up. If the taint is not removed within this duration,
	// the watcher stops to avoid blocking indefinitely.
	TaintWatcherDuration = 10 * time.Minute
)

// JSONPatch represents a single JSON Patch operation (RFC 6902).
type JSONPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// StartNotReadyTaintWatcher watches the local node and removes the
// agent-not-ready taint once the CSI driver has registered with kubelet
// (verified via CSINode object). This prevents workload pods from being
// scheduled before the driver is ready, avoiding "driver not found" errors
// during node startup, reboot, or autoscaling events.
func StartNotReadyTaintWatcher(clientset kubernetes.Interface, nodeID string, maxDuration time.Duration) {
	klog.Infof("Starting taint watcher for node %s (taint key: %s, timeout: %v)", nodeID, AgentNotReadyNodeTaintKey, maxDuration)

	ctx, cancel := context.WithTimeout(context.Background(), maxDuration)
	defer cancel()

	// Create a node informer filtered to just this node
	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		0, // no resync
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", nodeID).String()
		}),
	)

	nodeInformer := factory.Core().V1().Nodes().Informer()
	csiNodeInformer := factory.Storage().V1().CSINodes().Informer()
	stopCh := make(chan struct{})

	// Stop informers when we're done
	defer close(stopCh)

	// getNode retrieves the node from the informer cache (or directly from the API).
	getNode := func() *corev1.Node {
		obj, exists, err := nodeInformer.GetStore().GetByKey(nodeID)
		if err != nil || !exists {
			return nil
		}
		n, ok := obj.(*corev1.Node)
		if !ok {
			return nil
		}
		return n
	}

	// checkAndRemoveTaint is called on both Node and CSINode events.
	checkAndRemoveTaint := func() {
		n := getNode()
		if n == nil {
			return
		}
		handleNodeEvent(ctx, clientset, n, nodeID, cancel)
	}

	nodeHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { checkAndRemoveTaint() },
		UpdateFunc: func(_, _ any) { checkAndRemoveTaint() },
	}
	csiNodeHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { checkAndRemoveTaint() },
		UpdateFunc: func(_, _ any) { checkAndRemoveTaint() },
	}

	if _, err := nodeInformer.AddEventHandler(nodeHandler); err != nil {
		klog.Errorf("Taint watcher: failed to add node event handler: %v", err)
		return
	}
	if _, err := csiNodeInformer.AddEventHandler(csiNodeHandler); err != nil {
		klog.Errorf("Taint watcher: failed to add CSINode event handler: %v", err)
		return
	}

	factory.Start(stopCh)

	// Wait for both caches to sync
	if !cache.WaitForCacheSync(ctx.Done(), nodeInformer.HasSynced, csiNodeInformer.HasSynced) {
		klog.Errorf("Taint watcher: failed to sync informer caches for node %s", nodeID)
		return
	}

	// Wait until context is done (either taint removed or timeout)
	<-ctx.Done()
	if ctx.Err() == context.DeadlineExceeded {
		klog.Warningf("Taint watcher: timed out after %v waiting for driver registration on node %s", maxDuration, nodeID)
	}
}

// handleNodeEvent processes a node event, checking if the taint is present
// and the driver is registered, then removing the taint if appropriate.
func handleNodeEvent(ctx context.Context, clientset kubernetes.Interface, n *corev1.Node, nodeID string, cancel context.CancelFunc) {
	if !hasNotReadyTaint(n) {
		klog.V(4).Infof("Taint watcher: node %s does not have taint %s, stopping", nodeID, AgentNotReadyNodeTaintKey)
		cancel()
		return
	}

	if err := checkDriverRegistered(ctx, clientset, nodeID); err != nil {
		klog.V(4).Infof("Taint watcher: driver not yet registered on node %s: %v", nodeID, err)
		return
	}

	klog.Infof("Taint watcher: CSI driver registered on node %s, removing taint %s", nodeID, AgentNotReadyNodeTaintKey)

	// Retry taint removal with exponential backoff
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2.0,
		Steps:    5,
		Cap:      10 * time.Second,
	}

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		if err := removeNotReadyTaint(ctx, clientset, nodeID); err != nil {
			klog.Warningf("Taint watcher: failed to remove taint from node %s, will retry: %v", nodeID, err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		klog.Errorf("Taint watcher: exhausted retries removing taint from node %s: %v", nodeID, err)
		return
	}

	klog.Infof("Taint watcher: successfully removed taint %s from node %s", AgentNotReadyNodeTaintKey, nodeID)
	cancel()
}

// hasNotReadyTaint returns true if the node has the agent-not-ready taint.
func hasNotReadyTaint(n *corev1.Node) bool {
	for _, taint := range n.Spec.Taints {
		if taint.Key == AgentNotReadyNodeTaintKey {
			return true
		}
	}
	return false
}

// checkDriverRegistered verifies the CSI driver is registered on the node
// by checking the CSINode object for the driver name.
func checkDriverRegistered(ctx context.Context, clientset kubernetes.Interface, nodeID string) error {
	csiNode, err := clientset.StorageV1().CSINodes().Get(ctx, nodeID, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("CSINode %q not found", nodeID)
		}
		return fmt.Errorf("failed to get CSINode %q: %w", nodeID, err)
	}

	for _, driver := range csiNode.Spec.Drivers {
		if driver.Name == constants.DriverName {
			return nil
		}
	}

	return fmt.Errorf("driver %q not found in CSINode %q (registered drivers: %v)",
		constants.DriverName, nodeID, driverNames(csiNode))
}

// driverNames returns the list of driver names from a CSINode for logging.
func driverNames(csiNode *storagev1.CSINode) []string {
	names := make([]string, 0, len(csiNode.Spec.Drivers))
	for _, d := range csiNode.Spec.Drivers {
		names = append(names, d.Name)
	}
	return names
}

// removeNotReadyTaint removes the agent-not-ready taint from the node
// using an atomic JSON Patch with a test-and-replace approach.
func removeNotReadyTaint(ctx context.Context, clientset kubernetes.Interface, nodeID string) error {
	if clientset == nil {
		return nil
	}

	// Get the current node to find the taint index
	n, err := clientset.CoreV1().Nodes().Get(ctx, nodeID, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %q: %w", nodeID, err)
	}

	taintIdx := -1
	for i, taint := range n.Spec.Taints {
		if taint.Key == AgentNotReadyNodeTaintKey {
			taintIdx = i
			break
		}
	}

	// No matching taint found — nothing to do
	if taintIdx == -1 {
		return nil
	}

	// Build atomic JSON Patch: test that the taint at the index is still ours, then remove it
	patches := []JSONPatch{
		{
			Op:    "test",
			Path:  fmt.Sprintf("/spec/taints/%d/key", taintIdx),
			Value: AgentNotReadyNodeTaintKey,
		},
		{
			Op:   "remove",
			Path: fmt.Sprintf("/spec/taints/%d", taintIdx),
		},
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = clientset.CoreV1().Nodes().Patch(ctx, nodeID, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node %q: %w", nodeID, err)
	}

	return nil
}

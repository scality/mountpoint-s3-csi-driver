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
	"fmt"
	"testing"
	"time"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestHasNotReadyTaint(t *testing.T) {
	tests := []struct {
		name     string
		taints   []corev1.Taint
		expected bool
	}{
		{
			name: "node with agent-not-ready taint",
			taints: []corev1.Taint{
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
			},
			expected: true,
		},
		{
			name:     "node with no taints",
			taints:   nil,
			expected: false,
		},
		{
			name: "node with other taints only",
			taints: []corev1.Taint{
				{Key: "node.kubernetes.io/not-ready", Effect: corev1.TaintEffectNoSchedule},
				{Key: "node.kubernetes.io/disk-pressure", Effect: corev1.TaintEffectNoSchedule},
			},
			expected: false,
		},
		{
			name: "node with multiple taints including agent-not-ready",
			taints: []corev1.Taint{
				{Key: "node.kubernetes.io/not-ready", Effect: corev1.TaintEffectNoSchedule},
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
				{Key: "node.kubernetes.io/disk-pressure", Effect: corev1.TaintEffectNoSchedule},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{
				Spec: corev1.NodeSpec{
					Taints: tt.taints,
				},
			}
			if got := hasNotReadyTaint(node); got != tt.expected {
				t.Errorf("hasNotReadyTaint() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCheckDriverRegistered(t *testing.T) {
	const nodeID = "test-node"

	tests := []struct {
		name      string
		csiNode   *storagev1.CSINode
		expectErr bool
	}{
		{
			name: "driver registered",
			csiNode: &storagev1.CSINode{
				ObjectMeta: metav1.ObjectMeta{Name: nodeID},
				Spec: storagev1.CSINodeSpec{
					Drivers: []storagev1.CSINodeDriver{
						{Name: constants.DriverName},
					},
				},
			},
			expectErr: false,
		},
		{
			name:      "CSINode not found",
			csiNode:   nil,
			expectErr: true,
		},
		{
			name: "no drivers registered",
			csiNode: &storagev1.CSINode{
				ObjectMeta: metav1.ObjectMeta{Name: nodeID},
				Spec:       storagev1.CSINodeSpec{Drivers: []storagev1.CSINodeDriver{}},
			},
			expectErr: true,
		},
		{
			name: "other drivers only",
			csiNode: &storagev1.CSINode{
				ObjectMeta: metav1.ObjectMeta{Name: nodeID},
				Spec: storagev1.CSINodeSpec{
					Drivers: []storagev1.CSINodeDriver{
						{Name: "ebs.csi.aws.com"},
						{Name: "efs.csi.aws.com"},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []runtime.Object
			if tt.csiNode != nil {
				objects = append(objects, tt.csiNode)
			}
			clientset := fake.NewSimpleClientset(objects...)

			err := checkDriverRegistered(context.Background(), clientset, nodeID)
			if tt.expectErr && err == nil {
				t.Error("checkDriverRegistered() expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("checkDriverRegistered() unexpected error: %v", err)
			}
		})
	}
}

func TestRemoveNotReadyTaint(t *testing.T) {
	const nodeID = "test-node"

	tests := []struct {
		name           string
		taints         []corev1.Taint
		expectPatch    bool
		expectErr      bool
		remainingCount int // expected number of taints after removal
	}{
		{
			name: "taint present and removed",
			taints: []corev1.Taint{
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
			},
			expectPatch:    true,
			remainingCount: 0,
		},
		{
			name:           "no matching taint is a no-op",
			taints:         []corev1.Taint{{Key: "other-taint", Effect: corev1.TaintEffectNoSchedule}},
			expectPatch:    false,
			remainingCount: 1,
		},
		{
			name: "multiple taints only removes ours",
			taints: []corev1.Taint{
				{Key: "node.kubernetes.io/not-ready", Effect: corev1.TaintEffectNoSchedule},
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
				{Key: "node.kubernetes.io/disk-pressure", Effect: corev1.TaintEffectNoSchedule},
			},
			expectPatch:    true,
			remainingCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: nodeID},
				Spec:       corev1.NodeSpec{Taints: tt.taints},
			}

			clientset := fake.NewSimpleClientset(node)

			var patchCalled bool
			clientset.PrependReactor("patch", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
				patchCalled = true
				// Let the fake client handle the patch
				return false, nil, nil
			})

			err := removeNotReadyTaint(context.Background(), clientset, nodeID)

			if tt.expectErr && err == nil {
				t.Error("removeNotReadyTaint() expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("removeNotReadyTaint() unexpected error: %v", err)
			}
			if tt.expectPatch != patchCalled {
				t.Errorf("removeNotReadyTaint() patch called = %v, want %v", patchCalled, tt.expectPatch)
			}
		})
	}
}

func TestRemoveNotReadyTaintNilClientset(t *testing.T) {
	err := removeNotReadyTaint(context.Background(), nil, "test-node")
	if err != nil {
		t.Errorf("removeNotReadyTaint(nil clientset) unexpected error: %v", err)
	}
}

func TestHandleNodeEvent_NoTaint(t *testing.T) {
	const nodeID = "test-node"
	clientset := fake.NewSimpleClientset()

	cancelled := false
	cancel := context.CancelFunc(func() { cancelled = true })

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec:       corev1.NodeSpec{},
	}

	handleNodeEvent(context.Background(), clientset, node, nodeID, cancel)

	if !cancelled {
		t.Error("expected cancel to be called when node has no taint")
	}
}

func TestHandleNodeEvent_TaintPresentDriverNotRegistered(t *testing.T) {
	const nodeID = "test-node"
	clientset := fake.NewSimpleClientset()

	cancelled := false
	cancel := context.CancelFunc(func() { cancelled = true })

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
			},
		},
	}

	handleNodeEvent(context.Background(), clientset, node, nodeID, cancel)

	if cancelled {
		t.Error("expected cancel NOT to be called when driver is not registered")
	}
}

func TestHandleNodeEvent_TaintPresentDriverRegistered(t *testing.T) {
	const nodeID = "test-node"

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
			},
		},
	}
	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{Name: constants.DriverName},
			},
		},
	}

	clientset := fake.NewSimpleClientset(node, csiNode)

	cancelled := false
	cancel := context.CancelFunc(func() { cancelled = true })

	handleNodeEvent(context.Background(), clientset, node, nodeID, cancel)

	if !cancelled {
		t.Error("expected cancel to be called after taint removal")
	}

	patchFound := false
	for _, action := range clientset.Actions() {
		if action.GetVerb() == "patch" && action.GetResource().Resource == "nodes" {
			patchFound = true
			break
		}
	}
	if !patchFound {
		t.Error("expected a patch action to remove the taint")
	}
}

func TestHandleNodeEvent_RemovalRetriesExhausted(t *testing.T) {
	const nodeID = "test-node"

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
			},
		},
	}
	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{Name: constants.DriverName},
			},
		},
	}

	clientset := fake.NewSimpleClientset(node, csiNode)
	clientset.PrependReactor("patch", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("simulated patch failure")
	})

	cancelled := false
	cancel := context.CancelFunc(func() { cancelled = true })

	handleNodeEvent(context.Background(), clientset, node, nodeID, cancel)

	if cancelled {
		t.Error("expected cancel NOT to be called when taint removal fails")
	}
}

func TestStartNotReadyTaintWatcher_RemovesTaintWhenDriverRegistered(t *testing.T) {
	const nodeID = "test-node"

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
			},
		},
	}
	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{Name: constants.DriverName},
			},
		},
	}

	clientset := fake.NewSimpleClientset(node, csiNode)

	done := make(chan struct{})
	go func() {
		StartNotReadyTaintWatcher(clientset, nodeID, 5*time.Second)
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("StartNotReadyTaintWatcher did not complete within timeout")
	}
}

func TestStartNotReadyTaintWatcher_ExitsWhenNoTaint(t *testing.T) {
	const nodeID = "test-node"

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec:       corev1.NodeSpec{},
	}

	clientset := fake.NewSimpleClientset(node)

	done := make(chan struct{})
	go func() {
		StartNotReadyTaintWatcher(clientset, nodeID, 5*time.Second)
		close(done)
	}()

	select {
	case <-done:
		// Success - exited quickly
	case <-time.After(3 * time.Second):
		t.Fatal("expected watcher to exit quickly when node has no taint")
	}
}

func TestStartNotReadyTaintWatcher_CSINodeRegistrationAfterStart(t *testing.T) {
	const nodeID = "test-node"

	// Node has taint, but CSINode does NOT exist yet — simulates the real
	// startup race where the driver registers with kubelet after the
	// taint watcher has already started watching.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
			},
		},
	}

	clientset := fake.NewSimpleClientset(node)

	done := make(chan struct{})
	go func() {
		StartNotReadyTaintWatcher(clientset, nodeID, 10*time.Second)
		close(done)
	}()

	// Give the informers time to start and process the initial list
	time.Sleep(500 * time.Millisecond)

	// Now create the CSINode — this simulates kubelet registering the driver
	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{Name: constants.DriverName},
			},
		},
	}
	if _, err := clientset.StorageV1().CSINodes().Create(context.Background(), csiNode, metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create CSINode: %v", err)
	}

	select {
	case <-done:
		// Success — CSINode informer triggered taint removal
	case <-time.After(5 * time.Second):
		t.Fatal("taint was not removed after CSINode registration")
	}
}

func TestStartNotReadyTaintWatcher_TimesOut(t *testing.T) {
	const nodeID = "test-node"

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
			},
		},
	}

	// No CSINode — driver never registers
	clientset := fake.NewSimpleClientset(node)

	start := time.Now()
	StartNotReadyTaintWatcher(clientset, nodeID, 1*time.Second)
	elapsed := time.Since(start)

	if elapsed < 900*time.Millisecond {
		t.Errorf("expected watcher to run for at least 900ms, got %v", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("expected watcher to timeout within 5s, got %v", elapsed)
	}
}

func TestRemoveNotReadyTaintPatchFailure(t *testing.T) {
	const nodeID = "test-node"
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeID},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute},
			},
		},
	}

	clientset := fake.NewSimpleClientset(node)
	clientset.PrependReactor("patch", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, context.DeadlineExceeded
	})

	err := removeNotReadyTaint(context.Background(), clientset, nodeID)
	if err == nil {
		t.Error("removeNotReadyTaint() expected error on patch failure, got nil")
	}
}

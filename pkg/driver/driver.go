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

package driver

import (
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/container-storage-interface/spec/lib/go/csi"
	crdv2 "github.com/scality/mountpoint-s3-csi-driver/pkg/api/v2"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	controllerCredProvider "github.com/scality/mountpoint-s3-csi-driver/pkg/driver/controller/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/envprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/mounter"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/version"
	mppodmounter "github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint/mounter"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod/watcher"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/s3client"
	"google.golang.org/grpc"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsclientsetscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

// Package-level scheme for controller-runtime operations
var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(crdv2.AddToScheme(scheme))
	utilruntime.Must(apiextensionsclientsetscheme.AddToScheme(scheme))
}

const (
	driverName = constants.DriverName

	grpcServerMaxReceiveMessageSize = 1024 * 1024 * 2 // 2MB

	unixSocketPerm = os.FileMode(0o700) // only owner can write and read.

	podWatcherResyncPeriod = time.Minute
)

var mountpointPodNamespace = os.Getenv("MOUNTPOINT_NAMESPACE")

// Test seams: allow overriding external dependencies in unit tests.
var (
	inClusterConfigFn        = rest.InClusterConfig
	newKubernetesForConfigFn = func(c *rest.Config) (kubernetes.Interface, error) { return kubernetes.NewForConfig(c) }
	kubernetesVersionFn      = kubernetesVersion
	checkSelectableFieldsFn  = checkIfMountpointS3PodAttachmentHasNodeNameSelectableFieldInCurrentVersion
	setupCacheFn             = setupS3PodAttachmentCache
)

// InClusterConfigTestHook allows tests to override the in-cluster config function.
// Pass nil to restore the default behavior.
func InClusterConfigTestHook(hook func() (*rest.Config, error)) {
	if hook == nil {
		inClusterConfigFn = rest.InClusterConfig
		return
	}
	inClusterConfigFn = hook
}

// KubeClientForConfigTestHook allows tests to override the Kubernetes client creation.
// Pass nil to restore the default behavior.
func KubeClientForConfigTestHook(hook func(*rest.Config) (kubernetes.Interface, error)) {
	if hook == nil {
		newKubernetesForConfigFn = func(c *rest.Config) (kubernetes.Interface, error) { return kubernetes.NewForConfig(c) }
		return
	}
	newKubernetesForConfigFn = hook
}

// KubernetesVersionTestHook allows tests to override Kubernetes version detection.
// Pass nil to restore the default behavior.
func KubernetesVersionTestHook(hook func(kubernetes.Interface) (string, error)) {
	if hook == nil {
		kubernetesVersionFn = kubernetesVersion
		return
	}
	kubernetesVersionFn = hook
}

// CheckSelectableFieldsTestHook allows tests to override CRD selectable fields check.
// Pass nil to restore the default behavior.
func CheckSelectableFieldsTestHook(hook func(context.Context, *rest.Config) (bool, error)) {
	if hook == nil {
		checkSelectableFieldsFn = checkIfMountpointS3PodAttachmentHasNodeNameSelectableFieldInCurrentVersion
		return
	}
	checkSelectableFieldsFn = hook
}

// SetupCacheTestHook allows tests to override cache setup.
// Pass nil to restore the default behavior.
func SetupCacheTestHook(hook func(*rest.Config, <-chan struct{}, string, string) ctrlcache.Cache) {
	if hook == nil {
		setupCacheFn = setupS3PodAttachmentCache
		return
	}
	setupCacheFn = hook
}

// setupS3PodAttachmentCache sets up cache for MountpointS3PodAttachment custom resource
// following AWS's production-grade implementation with mandatory cache and field selector detection
func setupS3PodAttachmentCache(config *rest.Config, stopCh <-chan struct{}, nodeID, kubernetesVersion string) ctrlcache.Cache {
	// Create a variable to hold the resync period for addressability
	syncPeriod := podWatcherResyncPeriod
	options := ctrlcache.Options{
		Scheme:                      scheme,
		SyncPeriod:                  &syncPeriod,
		ReaderFailOnMissingInformer: true,
	}

	// Check if the cluster supports field selectors for spec.nodeName
	ctx := context.Background()
	isSelectFieldsSupported, err := checkSelectableFieldsFn(ctx, config)
	if err != nil {
		klog.Fatalf("Failed to check support for selectable fields in the cluster: %v", err)
	}

	if isSelectFieldsSupported {
		klog.Info("Using `spec.nodeName` filter for caching MountpointS3PodAttachment as the cluster supports it")
		options.ByObject = map[client.Object]ctrlcache.ByObject{
			&crdv2.MountpointS3PodAttachment{}: {
				Field: fields.OneTermEqualSelector("spec.nodeName", nodeID),
			},
		}
	} else {
		klog.Info("Cluster doesn't support selectable fields, falling back to client-side filtering")
		// Client-side filtering - cache all but filter in application
		options.ByObject = map[client.Object]ctrlcache.ByObject{
			&crdv2.MountpointS3PodAttachment{}: {},
		}
	}

	// Create the cache - fail-fast if it cannot be created
	s3paCache, err := ctrlcache.New(config, options)
	if err != nil {
		klog.Fatalf("Failed to create cache: %v", err)
	}

	// Setup field indices for fast lookups
	if err := crdv2.SetupCacheIndices(s3paCache); err != nil {
		klog.Fatalf("Failed to setup field indexers: %v", err)
	}

	// Get the informer to verify sync
	s3podAttachmentInformer, err := s3paCache.GetInformer(context.Background(), &crdv2.MountpointS3PodAttachment{})
	if err != nil {
		klog.Fatalf("Failed to create informer for MountpointS3PodAttachment: %v", err)
	}

	// Start the cache with signal handler for graceful shutdown
	go func() {
		if err := s3paCache.Start(signals.SetupSignalHandler()); err != nil {
			klog.Fatalf("Failed to start cache: %v", err)
		}
	}()

	// Wait for cache to sync - fail if it doesn't sync
	if !cache.WaitForCacheSync(stopCh, s3podAttachmentInformer.HasSynced) {
		klog.Fatalf("Failed to sync informer cache within the timeout")
	}

	klog.Infof("S3PodAttachment cache initialized for node %s (Kubernetes %s)", nodeID, kubernetesVersion)
	return s3paCache
}

// checkIfMountpointS3PodAttachmentHasNodeNameSelectableFieldInCurrentVersion returns whether
// MountpointS3PodAttachment CRD definition contains `spec.nodeName` as a `selectableField` in its current version.
func checkIfMountpointS3PodAttachmentHasNodeNameSelectableFieldInCurrentVersion(ctx context.Context, config *rest.Config) (bool, error) {
	client, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return false, fmt.Errorf("failed to create api extensions client: %w", err)
	}

	crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdv2.MountpointS3PodAttachmentsCRDName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get CRD %q: %w", crdv2.MountpointS3PodAttachmentsCRDName, err)
	}

	// Find the current version in the CRD spec
	idx := slices.IndexFunc(crd.Spec.Versions,
		func(version apiextensionsv1.CustomResourceDefinitionVersion) bool {
			return version.Name == crdv2.GroupVersion.Version
		})
	if idx == -1 {
		return false, fmt.Errorf("failed to find CRD version %q of %q", crdv2.GroupVersion.Version, crdv2.MountpointS3PodAttachmentsCRDName)
	}

	version := crd.Spec.Versions[idx]
	// Check if spec.nodeName is in the selectableFields
	return slices.ContainsFunc(version.SelectableFields, func(selectableField apiextensionsv1.SelectableField) bool {
		return selectableField.JSONPath == crdv2.SelectableFieldNodeNameJSONPath
	}), nil
}

type Driver struct {
	Endpoint string
	Srv      *grpc.Server
	NodeID   string

	NodeServer *node.S3NodeServer

	// Controller credential provider for dynamic provisioning
	controllerCredProvider *controllerCredProvider.Provider

	// Test S3 client factory for dependency injection in tests.
	// When set, this function is used instead of the real S3 client to enable
	// mocking during unit tests, preventing real S3 API calls in unit test scenarios.
	testS3ClientFactory func(context.Context, *aws.Config) (s3client.Client, error)

	stopCh chan struct{}

	// Embed the unimplemented servers to satisfy the interface
	csi.UnimplementedIdentityServer
	csi.UnimplementedControllerServer
}

func NewDriver(endpoint string, mpVersion string, nodeID string) (*Driver, error) {
	// Validate that AWS_ENDPOINT_URL is set
	if os.Getenv(envprovider.EnvEndpointURL) == "" {
		return nil, fmt.Errorf("AWS_ENDPOINT_URL environment variable must be set for the CSI driver to function")
	}

	config, err := inClusterConfigFn()
	if err != nil {
		return nil, fmt.Errorf("cannot create in-cluster config: %w", err)
	}

	clientset, err := newKubernetesForConfigFn(config)
	if err != nil {
		return nil, fmt.Errorf("cannot create kubernetes clientset: %w", err)
	}

	kubernetesVersion, err := kubernetesVersionFn(clientset)
	if err != nil {
		klog.Errorf("failed to get kubernetes version: %v", err)
	}

	version := version.GetVersion()
	klog.Infof("Driver version: %v, Git commit: %v, build date: %v, nodeID: %v, mount-s3 version: %v, kubernetes version: %v",
		version.DriverVersion, version.GitCommit, version.BuildDate, nodeID, mpVersion, kubernetesVersion)

	credProvider := credentialprovider.New(clientset.CoreV1())

	stopCh := make(chan struct{})

	var mounterImpl mounter.Mounter

	// Check if running in controller-only mode
	if os.Getenv("CSI_CONTROLLER_ONLY") == "true" {
		klog.Infoln("Running in controller-only mode, skipping mounter initialization")
		// No mounter needed for controller-only mode
		mounterImpl = nil
	} else {
		// Always use pod mounter (v2 only supports pod mounter)
		// Pass nodeID to watcher to filter pods scheduled on this node only
		podWatcher := watcher.New(clientset, mountpointPodNamespace, nodeID, podWatcherResyncPeriod)
		err = podWatcher.Start(stopCh)
		if err != nil {
			klog.Fatalf("failed to start Pod watcher: %v\n", err)
		}

		// Setup S3PodAttachment cache - mandatory for production use
		// Fail-fast approach: if cache cannot be created, the driver should not start
		s3paCache := setupCacheFn(config, stopCh, nodeID, kubernetesVersion)

		// Create PodUnmounter for cleanup of dangling mounts
		// Use the mountpoint mounter which implements the required MountInterface
		mountpointMounter := mppodmounter.NewDefaultMounter()
		unmounter := mounter.NewPodUnmounter(nodeID, mountpointMounter, podWatcher, credProvider)

		// Register event handler for immediate cleanup when pods are updated
		// This enables immediate response to pod state changes
		_, err = podWatcher.AddEventHandler(cache.ResourceEventHandlerFuncs{
			UpdateFunc: unmounter.HandleMountpointPodUpdate,
		})
		if err != nil {
			klog.Errorf("Failed to register unmounter event handler: %v", err)
		}

		// Start periodic cleanup for dangling mounts
		// The cleanup runs every 2 minutes as defined in the pod unmounter
		go unmounter.StartPeriodicCleanup(stopCh)

		mounterImpl, err = mounter.NewPodMounter(podWatcher, credProvider, mount.New(""), nil, nil, kubernetesVersion, s3paCache)
		if err != nil {
			klog.Fatalf("Failed to create pod mounter: %v", err)
		}

		klog.Infoln("Using pod mounter with S3PodAttachment cache and unmounter")
	}

	var nodeServer *node.S3NodeServer
	if mounterImpl != nil {
		nodeServer = node.NewS3NodeServer(nodeID, mounterImpl)
	}

	// Initialize controller credential provider for dynamic provisioning
	controllerCredProvider := controllerCredProvider.New(clientset)

	return &Driver{
		Endpoint:               endpoint,
		NodeID:                 nodeID,
		NodeServer:             nodeServer,
		controllerCredProvider: controllerCredProvider,
		stopCh:                 stopCh,
	}, nil
}

// NewDriverForTests creates a new driver instance for testing purposes
// This allows tests to provide their own Kubernetes client and node server
func NewDriverForTests(endpoint, nodeID string, nodeServer *node.S3NodeServer, kubeClient kubernetes.Interface) *Driver {
	controllerCredProv := controllerCredProvider.New(kubeClient)

	return &Driver{
		Endpoint:               endpoint,
		NodeID:                 nodeID,
		NodeServer:             nodeServer,
		controllerCredProvider: controllerCredProv,
		stopCh:                 make(chan struct{}),
	}
}

func (d *Driver) Run() error {
	scheme, addr, err := ParseEndpoint(d.Endpoint)
	if err != nil {
		return err
	}

	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return err
	}

	if scheme == "unix" {
		// Go's `net` package does not support specifying permissions on Unix sockets it creates.
		// There are two ways to change permissions:
		// 	 - Using `syscall.Umask` before `net.Listen`
		//   - Calling `os.Chmod` after `net.Listen`
		// The first one is not nice because it affects all files created in the process,
		// the second one has a time-window where the permissions of Unix socket would depend on `umask`
		// between `net.Listen` and `os.Chmod`. Since we don't start accepting connections on the socket until
		// `grpc.Serve` call, we should be fine with `os.Chmod` option.
		// See https://github.com/golang/go/issues/11822#issuecomment-123850227.
		if err := os.Chmod(addr, unixSocketPerm); err != nil {
			klog.Errorf("failed to change permissions on unix socket %s: %v", addr, err)
			return fmt.Errorf("failed to change permissions on unix socket %s: %v", addr, err)
		}
	}

	logErr := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			klog.Errorf("GRPC error: %v", err)
		}
		return resp, err
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logErr),
		grpc.MaxRecvMsgSize(grpcServerMaxReceiveMessageSize),
	}
	d.Srv = grpc.NewServer(opts...)

	csi.RegisterIdentityServer(d.Srv, d)
	csi.RegisterControllerServer(d.Srv, d)
	if d.NodeServer != nil {
		csi.RegisterNodeServer(d.Srv, d.NodeServer)
	}

	klog.Infof("Listening for connections on address: %#v", listener.Addr())
	return d.Srv.Serve(listener)
}

func (d *Driver) Stop() {
	klog.Infof("Stopping server")
	if d.stopCh != nil {
		close(d.stopCh)
		d.stopCh = nil
	}
	if d.Srv != nil {
		d.Srv.Stop()
	}
}

func kubernetesVersion(clientset kubernetes.Interface) (string, error) {
	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("cannot get kubernetes server version: %w", err)
	}

	return version.String(), nil
}

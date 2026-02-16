package mounter

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crdv2 "github.com/scality/mountpoint-s3-csi-driver/pkg/api/v2"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/envprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/targetpath"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
	mpmounter "github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint/mounter"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mountoptions"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod/watcher"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util"
)

// targetDirPerm is the permission to use while creating target directory if its not exists.
const targetDirPerm = fs.FileMode(0o755)

// mountSyscall is the function that performs FUSE mount operation for S3 buckets.
// It mounts the S3 bucket to the target directory and returns the FUSE device file descriptor.
// This abstraction allows for dependency injection during testing.
// In production, the platform-native mount implementation is used.
// Parameters:
//   - target: The directory where S3 bucket will be mounted (source directory in our architecture)
//   - args: Mountpoint-s3 arguments for the mount operation
//
// Returns:
//   - fd: FUSE device file descriptor for passing to Mountpoint Pod
//   - err: Any error that occurred during mount
type mountSyscall func(target string, args mountpoint.Args) (fd int, err error)

// bindMountSyscall is the function that performs bind mount operation.
// It creates a bind mount from source (shared S3 mount) to target (container-specific path).
// This abstraction enables testing without actual mount operations.
// In production, uses standard Linux bind mount via mount-utils.
// Parameters:
//   - source: The source directory with mounted S3 bucket
//   - target: The target directory requested by the container
type bindMountSyscall func(source, target string) error

// A PodMounter is a [Mounter] that mounts Mountpoint on pre-created Kubernetes Pod running in the same node.
// It implements a source/bind mount architecture where:
// - S3 buckets are mounted to a "source" directory first (shared mount point)
// - Individual containers get bind mounts from the source to their target paths
// - This enables multiple containers to share the same S3 mount efficiently
// - All workloads use CRD-based coordination for mount sharing
type PodMounter struct {
	podWatcher        *watcher.Watcher
	mount             mount.Interface
	kubeletPath       string
	mountSyscall      mountSyscall
	bindMountSyscall  bindMountSyscall
	kubernetesVersion string
	credProvider      *credentialprovider.Provider
	k8sClient         client.Reader // Changed to Reader to support both client.Client and cache.Cache
	nodeName          string
}

// NewPodMounter creates a new [PodMounter] with given Kubernetes client.
// Parameters:
// - podWatcher: Watches for Mountpoint Pod status changes
// - credProvider: Manages AWS credentials for S3 access
// - mount: Interface for mount operations
// - mountSyscall: Custom mount syscall function (nil uses default)
// - bindMountSyscall: Custom bind mount function (nil uses default)
// - kubernetesVersion: K8s version for compatibility checks
// - k8sClient: Reader for CRD operations (can be client.Client or cache.Cache, required)
func NewPodMounter(podWatcher *watcher.Watcher, credProvider *credentialprovider.Provider, mount mount.Interface, mountSyscall mountSyscall, bindMountSyscall bindMountSyscall, kubernetesVersion string, k8sClient client.Reader) (*PodMounter, error) {
	kubeletPath := os.Getenv("KUBELET_PATH")
	if kubeletPath == "" {
		kubeletPath = "/var/lib/kubelet"
	}
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" && k8sClient != nil {
		// NODE_NAME is required only when using CRD mode (k8sClient is provided)
		return nil, fmt.Errorf("NODE_NAME environment variable must be set when using CRD mode")
	}
	return &PodMounter{
		podWatcher:        podWatcher,
		credProvider:      credProvider,
		mount:             mount,
		kubeletPath:       kubeletPath,
		mountSyscall:      mountSyscall,
		bindMountSyscall:  bindMountSyscall,
		kubernetesVersion: kubernetesVersion,
		k8sClient:         k8sClient,
		nodeName:          nodeName,
	}, nil
}

// waitForMountpointPodAttachment waits for a MountpointS3PodAttachment CRD to be created by the controller.
// It continuously polls until the CRD is found or the context times out.
//
// The CRD-based coordination enables:
// - Controller to determine optimal Mountpoint Pod placement
// - Sharing of Mountpoint Pods across multiple workload pods
// - Better resource utilization and scheduling decisions
func (pm *PodMounter) waitForMountpointPodAttachment(ctx context.Context, podID, volumeName, volumeID string, credentialCtx credentialprovider.ProvideContext, fsGroup string) (string, error) {
	if pm.k8sClient == nil {
		return "", fmt.Errorf("k8sClient is required for pod mounter operations")
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Build field filters for searching MountpointS3PodAttachments
	fieldFilters := client.MatchingFields{
		crdv2.FieldNodeName:             pm.nodeName,
		crdv2.FieldPersistentVolumeName: volumeName,
		crdv2.FieldVolumeID:             volumeID,
		crdv2.FieldWorkloadFSGroup:      fsGroup,
	}

	klog.V(4).Infof("Waiting for MountpointS3PodAttachment for podID=%s, volumeName=%s, volumeID=%s", podID, volumeName, volumeID)

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timed out waiting for MountpointS3PodAttachment: %w", ctx.Err())
		default:
		}

		s3paList := &crdv2.MountpointS3PodAttachmentList{}
		err := pm.k8sClient.List(ctx, s3paList, fieldFilters)
		if err != nil {
			klog.Errorf("Failed to list MountpointS3PodAttachments: %v", err)
			return "", err
		}

		for _, s3pa := range s3paList.Items {
			for mpPodName, attachments := range s3pa.Spec.MountpointS3PodAttachments {
				for _, attachment := range attachments {
					if attachment.WorkloadPodUID == podID {
						klog.V(4).Infof("Found MountpointS3PodAttachment %s with Mountpoint Pod %s", s3pa.Name, mpPodName)
						return mpPodName, nil
					}
				}
			}
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timed out waiting for MountpointS3PodAttachment: %w", ctx.Err())
		case <-time.After(2 * time.Second):
			// Poll every 2 seconds
		}
	}
}

// helpMessageForGettingControllerLogs returns a help message for getting controller logs.
func (pm *PodMounter) helpMessageForGettingControllerLogs() string {
	return "You can see the controller logs by running `kubectl logs -n kube-system -lapp=s3-csi-controller`."
}

// Mount mounts the given `bucketName` at the `target` path using provided credential context and Mountpoint arguments.
//
// Source/Bind Mount Architecture:
//  1. Wait for controller to assign a Mountpoint Pod via CRD
//  2. Mount S3 bucket to a "source" directory: /var/lib/kubelet/plugins/s3.csi.scality.com/mnt/<pod-name>
//  3. Create bind mount from source to target path requested by the container
//  4. Multiple containers can share the same source mount via different bind mounts
//
// Benefits:
// - Resource efficiency: One Mountpoint Pod serves multiple containers
// - Better scheduling: Controller can optimize Mountpoint Pod placement
// - Graceful upgrades: Existing workloads continue working during CSI upgrade
// - Mount reuse: Source mount persists across container restarts
//
// The source mount is only created once and reused for subsequent bind mounts.
// Credentials are always updated to ensure they remain current.
func (pm *PodMounter) Mount(ctx context.Context, bucketName string, target string, credentialCtx credentialprovider.ProvideContext, args mountpoint.Args, fsGroup string) error {
	// Check if target is an existing systemd mountpoint (for seamless upgrade)
	// Only preserve systemd mounts if the mount is still active and accessible
	if pm.IsSystemDMountpoint(target) {
		// Check if the mount is still accessible
		if _, err := os.Stat(target); err == nil {
			klog.Infof("Target %q is an active SystemD mountpoint. Will only refresh credentials for seamless upgrade.", target)

			// Use the host plugin directory for systemd credential path
			// This matches where systemd mounter would have stored credentials
			hostPluginDir := filepath.Join(pm.kubeletPath, "plugins", constants.DriverName)
			credentialCtx.SetWriteAndEnvPath(hostPluginDir, hostPluginDir)

			// Only refresh credentials, don't attempt to remount
			_, _, err := pm.credProvider.Provide(ctx, credentialCtx)
			if err != nil {
				klog.Errorf("Failed to provide SystemD credentials for %q: %v", target, err)
				return fmt.Errorf("failed to provide SystemD credentials: %w", err)
			}

			klog.Infof("Successfully refreshed credentials for existing SystemD mount at %q", target)
			return nil // Early return - preserve existing systemd mount
		}
		// If the mount is not accessible (e.g., pod restarted with new UID),
		// fall through to create a new pod-based mount
		klog.Infof("SystemD mount at %q is no longer accessible, transitioning to pod mounter", target)
	}

	// For new mounts after upgrade, ensure the target directory has proper permissions
	// This fixes permission issues when transitioning from systemd to pod mounter
	if util.SupportLegacySystemdMounts() {
		// Ensure target directory exists and has proper permissions
		if err := os.MkdirAll(target, 0o755); err != nil && !os.IsExist(err) {
			klog.V(4).Infof("Failed to create target directory %s: %v", target, err)
		}
		// Try to fix permissions if they're wrong (best effort)
		if err := os.Chmod(target, 0o755); err != nil {
			klog.V(4).Infof("Failed to chmod target directory %s: %v (continuing anyway)", target, err)
		}
	}

	volumeName, err := pm.volumeNameFromTargetPath(target)
	if err != nil {
		return fmt.Errorf("failed to extract volume name from %q: %w", target, err)
	}

	podID := credentialCtx.PodID
	volumeID := credentialCtx.VolumeID

	// Step 1: Determine which Mountpoint Pod to use via MountpointS3PodAttachment CRD
	// Controller assigns optimal pod based on scheduling and resource constraints
	klog.V(4).Infof("Looking for pod with podID=%s, volumeName=%s, volumeID=%s", podID, volumeName, volumeID)
	mpPodName, err := pm.waitForMountpointPodAttachment(ctx, podID, volumeName, volumeID, credentialCtx, fsGroup)
	if err != nil {
		klog.Errorf("failed to wait for MountpointS3PodAttachment for %q: %v. %s", target, err, pm.helpMessageForGettingControllerLogs())
		return fmt.Errorf("failed to wait for MountpointS3PodAttachment for %q: %w. %s", target, err, pm.helpMessageForGettingControllerLogs())
	}
	klog.V(4).Infof("Using Mountpoint Pod name: %s", mpPodName)

	// Step 2: Setup source mount directory
	// Source path: /var/lib/kubelet/plugins/s3.csi.scality.com/mnt/<mp-pod-name>
	// This is where S3 bucket will be mounted (shared across containers)
	source := filepath.Join(SourceMountDir(pm.kubeletPath), mpPodName)

	// Verify source mount directory can be used
	err = pm.verifyOrSetupMountTarget(source)
	if err != nil {
		return fmt.Errorf("failed to verify source path can be used as a mount point %q: %w", source, err)
	}

	// Check if source is already mounted
	isSourceMounted, err := pm.IsMountPoint(source)
	if err != nil {
		return fmt.Errorf("could not check if source %q is already a mount point: %w", source, err)
	}

	// Check if target is already mounted (bind mount)
	err = pm.verifyOrSetupMountTarget(target)
	if err != nil {
		return fmt.Errorf("failed to verify target path can be used as a mount point %q: %w", target, err)
	}

	isTargetMounted, err := pm.IsMountPoint(target)
	if err != nil {
		return fmt.Errorf("could not check if target %q is already a mount point: %w", target, err)
	}

	pod, podPath, err := pm.waitForMountpointPod(ctx, mpPodName)
	if err != nil {
		klog.Errorf("failed to wait for Mountpoint Pod to be ready for %q: %v", target, err)
		return fmt.Errorf("failed to wait for Mountpoint Pod to be ready for %q: %w", target, err)
	}
	unlockMountpointPod := lockMountpointPod(mpPodName)
	defer unlockMountpointPod()

	podCredentialsPath, err := pm.ensureCredentialsDirExists(podPath)
	if err != nil {
		klog.Errorf("failed to create credentials directory for %q: %v", target, err)
		return fmt.Errorf("failed to create credentials directory for %q: %w", target, err)
	}

	credentialCtx.SetWriteAndEnvPath(podCredentialsPath, mppod.PathInsideMountpointPod(mppod.KnownPathCredentials))

	// Always provide credentials to ensure they're up-to-date
	credEnv, authenticationSource, err := pm.credProvider.Provide(ctx, credentialCtx)
	if err != nil {
		klog.Errorf("failed to provide credentials for %s: %v\n%s", target, err, pm.helpMessageForGettingMountpointLogs(pod))
		return fmt.Errorf("failed to provide credentials for %q: %w\n%s", target, err, pm.helpMessageForGettingMountpointLogs(pod))
	}

	// Step 3: Mount S3 bucket to source directory (if not already mounted)
	// This creates the shared mount point that multiple containers can use
	if !isSourceMounted {
		env := envprovider.Default()
		env.Merge(credEnv)

		// Move `--aws-max-attempts` to env if provided
		if maxAttempts, ok := args.Remove(mountpoint.ArgAWSMaxAttempts); ok {
			env.Set(envprovider.EnvMaxAttempts, maxAttempts)
		}

		enforceCSIDriverMountArgPolicy(&args)

		// Remove the read-only argument from the list as mount-s3 does not support it when using FUSE
		if args.Has(mountpoint.ArgReadOnly) {
			args.Remove(mountpoint.ArgReadOnly)
		}

		args.Set(mountpoint.ArgUserAgentPrefix, UserAgent(authenticationSource, pm.kubernetesVersion))
		podMountSockPath := mppod.PathOnHost(podPath, mppod.KnownPathMountSock)
		podMountErrorPath := mppod.PathOnHost(podPath, mppod.KnownPathMountError)

		klog.V(4).Infof("Mounting S3 bucket to source %s for %s", source, pod.Name)

		fuseDeviceFD, err := pm.mountSyscallWithDefault(source, args)
		if err != nil {
			klog.Errorf("failed to mount source %s: %v", source, err)
			return fmt.Errorf("failed to mount source %s: %w", source, err)
		}

		// This will set to false in the success condition. This is set to `true` by default to
		// ensure we don't leave `source` mounted if Mountpoint is not started to serve requests for it.
		unmountSource := true
		defer func() {
			if unmountSource {
				if err := pm.unmountTarget(source); err != nil {
					klog.V(4).ErrorS(err, "failed to unmount mounted source %s\n", source)
				} else {
					klog.V(4).Infof("Source %s unmounted successfully\n", source)
				}
			}
		}()

		// This function can either fail or successfully send mount options to Mountpoint Pod - in which
		// Mountpoint Pod will get its own fd referencing the same underlying file description.
		// In both case we need to close the fd in this process.
		defer mpmounter.CloseFUSEDevice(fuseDeviceFD)

		// Remove old mount error file if exists
		_ = os.Remove(podMountErrorPath)

		klog.V(4).Infof("Sending mount options to Mountpoint Pod %s on %s", pod.Name, podMountSockPath)

		err = mountoptions.Send(ctx, podMountSockPath, mountoptions.Options{
			Fd:         fuseDeviceFD,
			BucketName: bucketName,
			Args:       args.SortedList(),
			Env:        env.List(),
		})
		if err != nil {
			klog.Errorf("failed to send mount option to Mountpoint Pod %s for source %s: %v\n%s", pod.Name, source, err, pm.helpMessageForGettingMountpointLogs(pod))
			return fmt.Errorf("failed to send mount options to Mountpoint Pod %s for source %s: %w\n%s", pod.Name, source, err, pm.helpMessageForGettingMountpointLogs(pod))
		}

		err = pm.waitForMount(ctx, source, pod.Name, podMountErrorPath)
		if err != nil {
			klog.Errorf("failed to wait for Mountpoint Pod %s to be ready for source %s: %v\n%s", pod.Name, source, err, pm.helpMessageForGettingMountpointLogs(pod))
			return fmt.Errorf("failed to wait for Mountpoint Pod %s to be ready for source %s: %w\n%s", pod.Name, source, err, pm.helpMessageForGettingMountpointLogs(pod))
		}

		// Mountpoint successfully started at source, so don't unmount it
		unmountSource = false
		klog.V(4).Infof("Successfully mounted S3 bucket to source %s", source)
	} else {
		klog.V(4).Infof("Source %s is already mounted, reusing existing mount", source)
	}

	// Step 4: Create bind mount from source to target
	// Skip if target already has a bind mount (idempotency)
	if isTargetMounted {
		klog.V(4).Infof("Target path %q is already bind-mounted", target)
		return nil
	}

	// Create bind mount: source (shared S3 mount) -> target (container-specific path)
	// This allows the container to access S3 at its requested path while sharing
	// the underlying S3 mount with other containers
	klog.V(4).Infof("Creating bind mount from source %s to target %s", source, target)
	err = pm.bindMountSyscallWithDefault(source, target)
	if err != nil {
		klog.Errorf("failed to bind mount %q to target %q: %v", source, target, err)
		return fmt.Errorf("failed to bind mount %q to target %q: %w", source, target, err)
	}

	klog.V(4).Infof("Successfully created bind mount to target %s from source %s", target, source)
	return nil
}

// Unmount unmounts only the bind mount point at `target`.
//
// Important: This only removes the bind mount, NOT the source mount.
// The source mount at /var/lib/kubelet/plugins/s3.csi.scality.com/mnt/<pod-name>
// remains active because:
// - Other containers might still be using it
// - Preserving it avoids expensive S3 remounts for new containers
// - The controller manages source mount lifecycle separately
//
// Source mount cleanup happens when:
// - The Mountpoint Pod is terminated by the controller
// - No workload pods need the mount anymore
// - During node shutdown or driver restart
func (pm *PodMounter) Unmount(ctx context.Context, target string, credentialCtx credentialprovider.CleanupContext) error {
	// Only unmount the bind mount at target, preserve the shared source mount
	err := pm.unmountTarget(target)
	if err != nil {
		klog.Errorf("failed to unmount target %q: %v", target, err)
		return fmt.Errorf("failed to unmount target %q: %w", target, err)
	}

	klog.V(4).Infof("Target %q successfully unmounted (bind mount removed)", target)
	return nil
}

// IsMountPoint returns whether given `target` is a mount point.
// It checks for both mountpoint-s3 mounts and bind mounts.
func (pm *PodMounter) IsMountPoint(target string) (bool, error) {
	// First check if it's a mountpoint-s3 mount
	isMpMount, err := mpmounter.CheckMountpoint(pm.mount, target)
	if err != nil {
		return false, err
	}
	if isMpMount {
		return true, nil
	}

	// Also check if it's any other kind of mount (e.g., bind mount)
	// This is important because targets are typically bind mounts from source
	notMnt, err := pm.mount.IsLikelyNotMountPoint(target)
	if err != nil {
		// If the path doesn't exist, return false without error
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !notMnt, nil
}

// waitForMountpointPod waints until Mountpoint Pod for given `podID` and `volumeName` is in `Running` state.
// It returns found Mountpoint Pod and it's base directory.
func (pm *PodMounter) waitForMountpointPod(ctx context.Context, podName string) (*corev1.Pod, string, error) {
	pod, err := pm.podWatcher.Wait(ctx, podName)
	if err != nil {
		return nil, "", err
	}

	klog.V(4).Infof("Mountpoint Pod %s/%s is running with id %s", pod.Namespace, podName, pod.UID)

	return pod, pm.podPath(pod), nil
}

// waitForMount waits until Mountpoint is successfully mounted at `target`.
// It returns an error if Mountpoint fails to mount.
func (pm *PodMounter) waitForMount(parentCtx context.Context, target, podName, podMountErrorPath string) error {
	ctx, cancel := context.WithCancel(parentCtx)
	// Cancel at the end to ensure we cancel polling from goroutines.
	defer cancel()

	mountResultCh := make(chan error)

	klog.V(4).Infof("Waiting until Mountpoint Pod %s mounts on %s", podName, target)

	// Poll for mount error file
	go func() {
		_ = wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
			res, err := os.ReadFile(podMountErrorPath)
			if err != nil {
				return false, nil
			}

			mountResultCh <- fmt.Errorf("mountpoint Pod %s failed: %s", podName, res)
			return true, nil
		})
	}()

	// Poll for `IsMountPoint` check
	go func() {
		err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
			isMounted, err := pm.IsMountPoint(target)
			klog.V(5).Infof("Checking if %s is mount point: isMounted=%v, err=%v", target, isMounted, err)
			return isMounted, err
		})

		if err != nil {
			mountResultCh <- fmt.Errorf("failed to check if Mountpoint Pod %s mounted: %w", podName, err)
		} else {
			mountResultCh <- nil
		}
	}()

	err := <-mountResultCh
	if err == nil {
		klog.V(4).Infof("Mountpoint Pod %s mounted on %s", podName, target)
	} else {
		klog.V(4).Infof("Mountpoint Pod %s failed to mount on %s: %v", podName, target, err)
	}

	return err
}

// verifyOrSetupMountTarget checks target path for existence and corrupted mount error.
// If the target dir does not exists it tries to create it.
// If the target dir is corrupted (decided with `mount.IsCorruptedMnt`) it tries to unmount it to have a clean mount.
func (pm *PodMounter) verifyOrSetupMountTarget(target string) error {
	err := mpmounter.VerifyMountPoint(target)
	if err == nil {
		return nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		klog.V(5).Infof("Target path does not exists %s, trying to create", target)
		if err := os.MkdirAll(target, targetDirPerm); err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}

		return nil
	} else if mount.IsCorruptedMnt(err) {
		klog.V(4).Infof("Target path %q is a corrupted mount. Trying to unmount", target)
		if unmountErr := pm.unmountTarget(target); unmountErr != nil {
			klog.V(4).Infof("failed to unmount target path %q: %v, original failure of stat: %v", target, unmountErr, err)
			return fmt.Errorf("failed to unmount target path %q: %w, original failure of stat: %v", target, unmountErr, err)
		}

		return nil
	}

	return err
}

// ensureCredentialsDirExists ensures credentials dir for `podPath` is exists.
// It returns credentials dir and any error.
func (pm *PodMounter) ensureCredentialsDirExists(podPath string) (string, error) {
	credentialsBasepath := pm.credentialsDir(podPath)
	err := os.Mkdir(credentialsBasepath, credentialprovider.CredentialDirPerm)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		klog.V(4).Infof("failed to create credentials directory for pod %s: %v", podPath, err)
		return "", err
	}

	return credentialsBasepath, nil
}

// credentialsDir returns credentials dir for `podPath`.
func (pm *PodMounter) credentialsDir(podPath string) string {
	return mppod.PathOnHost(podPath, mppod.KnownPathCredentials)
}

// podPath returns `pod`'s basepath inside kubelet's path.
func (pm *PodMounter) podPath(pod *corev1.Pod) string {
	return filepath.Join(pm.kubeletPath, "pods", string(pod.UID))
}

// mountSyscallWithDefault delegates to `mountSyscall` if set, or fallbacks to platform-native `mountSyscallDefault`.
func (pm *PodMounter) mountSyscallWithDefault(target string, args mountpoint.Args) (int, error) {
	if pm.mountSyscall != nil {
		return pm.mountSyscall(target, args)
	}

	return pm.mountSyscallDefault(target, args)
}

// bindMountSyscallWithDefault delegates to `bindMountSyscall` if set, or fallbacks to platform-native bind mount.
func (pm *PodMounter) bindMountSyscallWithDefault(source, target string) error {
	if pm.bindMountSyscall != nil {
		return pm.bindMountSyscall(source, target)
	}

	// Default bind mount using mount-utils
	return pm.mount.Mount(source, target, "", []string{"bind"})
}

// unmountTarget calls `unmount` syscall on `target`.
func (pm *PodMounter) unmountTarget(target string) error {
	return mpmounter.UnmountTarget(pm.mount, target)
}

// volumeNameFromTargetPath tries to extract PersistentVolume's name from `target` path.
func (pm *PodMounter) volumeNameFromTargetPath(target string) (string, error) {
	tp, err := targetpath.Parse(target)
	if err != nil {
		return "", err
	}
	return tp.VolumeID, nil
}

func (pm *PodMounter) helpMessageForGettingMountpointLogs(pod *corev1.Pod) string {
	return fmt.Sprintf("You can see Mountpoint logs by running: `kubectl logs -n %s %s`. If the Mountpoint Pod already restarted, you can also pass `--previous` to get logs from the previous run.", pod.Namespace, pod.Name)
}

// GetPodWatcher returns the pod watcher instance
func (pm *PodMounter) GetPodWatcher() *watcher.Watcher {
	return pm.podWatcher
}

// GetCredentialProvider returns the credential provider instance
func (pm *PodMounter) GetCredentialProvider() *credentialprovider.Provider {
	return pm.credProvider
}

// IsSystemDMountpoint checks if the target is a systemd mount by looking at mount references.
// Systemd mounts are directly mounted without bind mounts, so they have no references.
// This is used to detect existing systemd mounts during upgrade from v1.x to v2.x.
func (pm *PodMounter) IsSystemDMountpoint(target string) bool {
	if !util.SupportLegacySystemdMounts() {
		return false
	}

	// Check if target is a mount point
	isMountPoint, err := pm.mount.IsMountPoint(target)
	if err != nil || !isMountPoint {
		return false
	}

	// Systemd mounts are direct mounts (no bind mount references)
	// Pod mounter uses bind mounts, so if we find references, it's not a systemd mount
	references, err := pm.mount.GetMountRefs(target)
	if err != nil {
		klog.Warningf("Failed to find references to mountpoint %s to detect systemd mounts. Assuming it is not systemd mountpoint: %v", target, err)
		return false
	}

	// If there are no other references, it's likely a direct systemd mount
	// Pod mounter would have at least one reference (the bind mount)
	return len(references) == 1 // Only the mount itself, no bind references
}

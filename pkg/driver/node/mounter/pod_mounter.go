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
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/envprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/targetpath"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
	mpmounter "github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint/mounter"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mountoptions"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod/watcher"
)

// targetDirPerm is the permission to use while creating target directory if its not exists.
const targetDirPerm = fs.FileMode(0o755)

// SourceMountDir returns the base directory for Mountpoint source mounts
func SourceMountDir(kubeletPath string) string {
	return filepath.Join(kubeletPath, "plugins", "s3.csi.scality.com", "mounts")
}

// mountSyscall is the function that performs `mount` operation for given `target` with given Mountpoint `args`.
// It returns mounted FUSE file descriptor as a result.
// This is mainly exposed for testing, in production platform-native function (`mountSyscallDefault`) will be used.
type mountSyscall func(target string, args mountpoint.Args) (fd int, err error)

// A PodMounter is a [Mounter] that mounts Mountpoint on pre-created Kubernetes Pod running in the same node.
type PodMounter struct {
	podWatcher        *watcher.Watcher
	mount             mount.Interface
	kubeletPath       string
	mountSyscall      mountSyscall
	kubernetesVersion string
	credProvider      *credentialprovider.Provider
	k8sClient         client.Client
	nodeName          string
}

// NewPodMounter creates a new [PodMounter] with given Kubernetes client.
func NewPodMounter(podWatcher *watcher.Watcher, credProvider *credentialprovider.Provider, mount mount.Interface, mountSyscall mountSyscall, kubernetesVersion string, k8sClient client.Client) (*PodMounter, error) {
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
		kubernetesVersion: kubernetesVersion,
		k8sClient:         k8sClient,
		nodeName:          nodeName,
	}, nil
}

// waitForMountpointPodAttachment waits for a MountpointS3PodAttachment CRD to be created by the controller.
// It continuously polls until the CRD is found or the context times out.
func (pm *PodMounter) waitForMountpointPodAttachment(ctx context.Context, podID, volumeName, volumeID string, credentialCtx credentialprovider.ProvideContext) (string, error) {
	if pm.k8sClient == nil {
		// Backward compatibility: if no k8s client, return the pod name directly
		// This allows the old reconciler to still work
		klog.Warningf("k8sClient is nil, returning pod name directly")
		return mppod.MountpointPodNameFor(podID, volumeName), nil
	}
	
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	
	// Build field filters for searching MountpointS3PodAttachments
	fieldFilters := client.MatchingFields{
		crdv2.FieldNodeName:             pm.nodeName,
		crdv2.FieldPersistentVolumeName: volumeName,
		crdv2.FieldVolumeID:             volumeID,
		crdv2.FieldAuthenticationSource: credentialCtx.AuthenticationSource,
	}
	
	if credentialCtx.AuthenticationSource == credentialprovider.AuthenticationSourcePod {
		fieldFilters[crdv2.FieldWorkloadNamespace] = credentialCtx.PodNamespace
		// TODO: Add ServiceAccountName when it's added to ProvideContext
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
// At high level, this method will:
//  1. Wait for Mountpoint Pod to be `Running`
//  2. Write credentials to Mountpoint Pod's credentials directory
//  3. Obtain a FUSE file descriptor
//  4. Call `mount` syscall with `target` and obtained FUSE file descriptor
//  5. Send mount options (including FUSE file descriptor) to Mountpoint Pod
//  6. Wait until Mountpoint successfully mounts at `target`
//
// If Mountpoint is already mounted at `target`, it will return early at step 2 to ensure credentials are up-to-date.
func (pm *PodMounter) Mount(ctx context.Context, bucketName string, target string, credentialCtx credentialprovider.ProvideContext, args mountpoint.Args) error {
	volumeName, err := pm.volumeNameFromTargetPath(target)
	if err != nil {
		return fmt.Errorf("failed to extract volume name from %q: %w", target, err)
	}

	podID := credentialCtx.PodID
	volumeID := credentialCtx.VolumeID

	err = pm.verifyOrSetupMountTarget(target)
	if err != nil {
		return fmt.Errorf("failed to verify target path can be used as a mount point %q: %w", target, err)
	}

	isMountPoint, err := pm.IsMountPoint(target)
	if err != nil {
		return fmt.Errorf("could not check if %q is already a mount point: %w", target, err)
	}

	// Wait for the controller to create the MountpointS3PodAttachment CRD and spawn the Mountpoint Pod
	mpPodName, err := pm.waitForMountpointPodAttachment(ctx, podID, volumeName, volumeID, credentialCtx)
	if err != nil {
		klog.Errorf("failed to wait for MountpointS3PodAttachment for %q: %v. %s", target, err, pm.helpMessageForGettingControllerLogs())
		return fmt.Errorf("failed to wait for MountpointS3PodAttachment for %q: %w. %s", target, err, pm.helpMessageForGettingControllerLogs())
	}

	// TODO: If `target` is a `systemd`-mounted Mountpoint, this would return an error,
	// but we should still update the credentials for it by calling `credProvider.Provide`.
	pod, podPath, err := pm.waitForMountpointPod(ctx, mpPodName)
	if err != nil {
		klog.Errorf("failed to wait for Mountpoint Pod to be ready for %q: %v", target, err)
		return fmt.Errorf("failed to wait for Mountpoint Pod to be ready for %q: %w", target, err)
	}

	podCredentialsPath, err := pm.ensureCredentialsDirExists(podPath)
	if err != nil {
		klog.Errorf("failed to create credentials directory for %q: %v", target, err)
		return fmt.Errorf("failed to create credentials directory for %q: %w", target, err)
	}

	credentialCtx.SetWriteAndEnvPath(podCredentialsPath, mppod.PathInsideMountpointPod(mppod.KnownPathCredentials))

	// Note that this part happens before `isMountPoint` check, as we want to update credentials even though
	// there is an existing mount point at `target`.
	credEnv, authenticationSource, err := pm.credProvider.Provide(ctx, credentialCtx)
	if err != nil {
		klog.Errorf("failed to provide credentials for %s: %v\n%s", target, err, pm.helpMessageForGettingMountpointLogs(pod))
		return fmt.Errorf("failed to provide credentials for %q: %w\n%s", target, err, pm.helpMessageForGettingMountpointLogs(pod))
	}

	if isMountPoint {
		klog.V(4).Infof("Target path %q is already mounted", target)
		return nil
	}

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

	klog.V(4).Infof("Mounting %s for %s", target, pod.Name)

	fuseDeviceFD, err := pm.mountSyscallWithDefault(target, args)
	if err != nil {
		klog.Errorf("failed to mount %s: %v", target, err)
		return fmt.Errorf("failed to mount %s: %w", target, err)
	}

	// This will set to false in the success condition. This is set to `true` by default to
	// ensure we don't leave `target` mounted if Mountpoint is not started to serve requests for it.
	unmount := true
	defer func() {
		if unmount {
			if err := pm.unmountTarget(target); err != nil {
				klog.V(4).ErrorS(err, "failed to unmount mounted target %s\n", target)
			} else {
				klog.V(4).Infof("Target %s unmounted successfully\n", target)
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
		klog.Errorf("failed to send mount option to Mountpoint Pod %s for %s: %v\n%s", pod.Name, target, err, pm.helpMessageForGettingMountpointLogs(pod))
		return fmt.Errorf("failed to send mount options to Mountpoint Pod %s for %s: %w\n%s", pod.Name, target, err, pm.helpMessageForGettingMountpointLogs(pod))
	}

	err = pm.waitForMount(ctx, target, pod.Name, podMountErrorPath)
	if err != nil {
		klog.Errorf("failed to wait for Mountpoint Pod %s to be ready for %s: %v\n%s", pod.Name, target, err, pm.helpMessageForGettingMountpointLogs(pod))
		return fmt.Errorf("failed to wait for Mountpoint Pod %s to be ready for %s: %w\n%s", pod.Name, target, err, pm.helpMessageForGettingMountpointLogs(pod))
	}

	// Mountpoint successfully started, so don't unmount the filesystem
	unmount = false
	return nil
}

// Unmount unmounts the mount point at `target` and cleans all credentials.
func (pm *PodMounter) Unmount(ctx context.Context, target string, credentialCtx credentialprovider.CleanupContext) error {
	volumeName, err := pm.volumeNameFromTargetPath(target)
	if err != nil {
		return fmt.Errorf("failed to extract volume name from %q: %w", target, err)
	}

	podID := credentialCtx.PodID
	mpPodName := mppod.MountpointPodNameFor(podID, volumeName)

	// TODO: If `target` is a `systemd`-mounted Mountpoint, this would return an error,
	// but we should still unmount it and clean the credentials.
	pod, podPath, err := pm.waitForMountpointPod(ctx, mpPodName)
	if err != nil {
		klog.Errorf("failed to wait for Mountpoint Pod to be ready for %q: %v", target, err)
		return fmt.Errorf("failed to wait for Mountpoint Pod for %q: %w", target, err)
	}

	credentialCtx.WritePath = pm.credentialsDir(podPath)

	// Write `mount.exit` file to indicate Mountpoint Pod to cleanly exit.
	podMountExitPath := mppod.PathOnHost(podPath, mppod.KnownPathMountExit)
	_, err = os.OpenFile(podMountExitPath, os.O_RDONLY|os.O_CREATE, credentialprovider.CredentialFilePerm)
	if err != nil {
		klog.Errorf("failed to send a exit message to Mountpoint Pod for %q: %s\n%s", target, err, pm.helpMessageForGettingMountpointLogs(pod))
		return fmt.Errorf("failed to send a exit message to Mountpoint Pod for %q: %w\n%s", target, err, pm.helpMessageForGettingMountpointLogs(pod))
	}

	err = pm.unmountTarget(target)
	if err != nil {
		klog.Errorf("failed to unmount %q: %v", target, err)
		return fmt.Errorf("failed to unmount %q: %w", target, err)
	}

	err = pm.credProvider.Cleanup(credentialCtx)
	if err != nil {
		klog.Errorf("failed to clean up credentials for %s: %v\n%s", target, err, pm.helpMessageForGettingMountpointLogs(pod))
		return fmt.Errorf("failed to clean up credentials for %q: %w\n%s", target, err, pm.helpMessageForGettingMountpointLogs(pod))
	}

	return nil
}

// IsMountPoint returns whether given `target` is a `mount-s3` mount.
func (pm *PodMounter) IsMountPoint(target string) (bool, error) {
	// TODO: Can we just use regular `IsMountPoint` check from `mounter` with containerization?
	return mpmounter.CheckMountpoint(pm.mount, target)
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
			return pm.IsMountPoint(target)
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

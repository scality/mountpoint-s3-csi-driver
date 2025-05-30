// WIP: Part of https://github.com/awslabs/mountpoint-s3-csi-driver/issues/279.
//
// `scality-csi-controller` is the entrypoint binary for the CSI Driver's controller component.
// It is responsible for acting on cluster events and spawning Mountpoint Pods when necessary.
// It is also responsible for managing Mountpoint Pods, for example it ensures that completed Mountpoint Pods gets deleted.
// It doesn't implement CSI's controller service as of today.
package main

import (
	"flag"
	"os"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/scality/mountpoint-s3-csi-driver/cmd/scality-csi-controller/csicontroller"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/cluster"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/version"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
)

var (
	mountpointNamespace         = flag.String("mountpoint-namespace", os.Getenv("MOUNTPOINT_NAMESPACE"), "Namespace to spawn Mountpoint Pods in.")
	mountpointVersion           = flag.String("mountpoint-version", os.Getenv("MOUNTPOINT_VERSION"), "Version of Mountpoint within the given Mountpoint image.")
	mountpointPriorityClassName = flag.String("mountpoint-priority-class-name", os.Getenv("MOUNTPOINT_PRIORITY_CLASS_NAME"), "Priority class name of the Mountpoint Pods.")
	mountpointImage             = flag.String("mountpoint-image", os.Getenv("MOUNTPOINT_IMAGE"), "Image of Mountpoint to use in spawned Mountpoint Pods.")
	mountpointImagePullPolicy   = flag.String("mountpoint-image-pull-policy", os.Getenv("MOUNTPOINT_IMAGE_PULL_POLICY"), "Pull policy of Mountpoint images.")
	mountpointContainerCommand  = flag.String("mountpoint-container-command", "/bin/scality-s3-csi-mounter", "Entrypoint command of the Mountpoint Pods.")
)

func main() {
	flag.Parse()

	logf.SetLogger(zap.New())

	log := logf.Log.WithName(csicontroller.Name)
	client := config.GetConfigOrDie()

	mgr, err := manager.New(client, manager.Options{})
	if err != nil {
		log.Error(err, "failed to create a new manager")
		os.Exit(1)
	}

	err = csicontroller.NewReconciler(mgr.GetClient(), mppod.Config{
		Namespace:         *mountpointNamespace,
		MountpointVersion: *mountpointVersion,
		PriorityClassName: *mountpointPriorityClassName,
		Container: mppod.ContainerConfig{
			Command:         *mountpointContainerCommand,
			Image:           *mountpointImage,
			ImagePullPolicy: corev1.PullPolicy(*mountpointImagePullPolicy),
		},
		CSIDriverVersion: version.GetVersion().DriverVersion,
		ClusterVariant:   cluster.DetectVariant(client, log),
	}).SetupWithManager(mgr)
	if err != nil {
		log.Error(err, "failed to create controller")
		os.Exit(1)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "failed to start manager")
		os.Exit(1)
	}
}

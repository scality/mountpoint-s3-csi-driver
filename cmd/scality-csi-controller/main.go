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
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/scality/mountpoint-s3-csi-driver/cmd/scality-csi-controller/csicontroller"
	crdv2 "github.com/scality/mountpoint-s3-csi-driver/pkg/api/v2"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/cluster"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/version"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
)

var (
	mountpointNamespace                    = flag.String("mountpoint-namespace", os.Getenv("MOUNTPOINT_NAMESPACE"), "Namespace to spawn Mountpoint Pods in.")
	mountpointVersion                      = flag.String("mountpoint-version", os.Getenv("MOUNTPOINT_VERSION"), "Version of Mountpoint within the given Mountpoint image.")
	mountpointPriorityClassName            = flag.String("mountpoint-priority-class-name", os.Getenv("MOUNTPOINT_PRIORITY_CLASS_NAME"), "Priority class name of the Mountpoint Pods.")
	mountpointPreemptingPriorityClassName  = flag.String("mountpoint-preempting-priority-class-name", os.Getenv("MOUNTPOINT_PREEMPTING_PRIORITY_CLASS_NAME"), "Preempting priority class name of the Mountpoint Pods.")
	mountpointHeadroomPriorityClassName    = flag.String("mountpoint-headroom-priority-class-name", os.Getenv("MOUNTPOINT_HEADROOM_PRIORITY_CLASS_NAME"), "Priority class name of the Headroom Pods.")
	mountpointImage                        = flag.String("mountpoint-image", os.Getenv("MOUNTPOINT_IMAGE"), "Image of Mountpoint to use in spawned Mountpoint Pods.")
	headroomImage                          = flag.String("headroom-image", os.Getenv("MOUNTPOINT_HEADROOM_IMAGE"), "Image of a pause container to use in spawned Headroom Pods.")
	mountpointImagePullPolicy              = flag.String("mountpoint-image-pull-policy", os.Getenv("MOUNTPOINT_IMAGE_PULL_POLICY"), "Pull policy of Mountpoint images.")
	mountpointContainerCommand             = flag.String("mountpoint-container-command", "/bin/scality-s3-csi-mounter", "Entrypoint command of the Mountpoint Pods.")
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(crdv2.AddToScheme(scheme))
}

func main() {
	flag.Parse()

	logf.SetLogger(zap.New())

	log := logf.Log.WithName(csicontroller.Name)
	conf := config.GetConfigOrDie()

	mgr, err := manager.New(conf, manager.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Error(err, "failed to create a new manager")
		os.Exit(1)
	}

	podConfig := mppod.Config{
		Namespace:                   *mountpointNamespace,
		MountpointVersion:           *mountpointVersion,
		PriorityClassName:           *mountpointPriorityClassName,
		PreemptingPriorityClassName: *mountpointPreemptingPriorityClassName,
		HeadroomPriorityClassName:   *mountpointHeadroomPriorityClassName,
		Container: mppod.ContainerConfig{
			Command:         *mountpointContainerCommand,
			Image:           *mountpointImage,
			HeadroomImage:   *headroomImage,
			ImagePullPolicy: corev1.PullPolicy(*mountpointImagePullPolicy),
		},
		CSIDriverVersion: version.GetVersion().DriverVersion,
		ClusterVariant:   cluster.DetectVariant(conf, log),
	}

	// Setup the original pod reconciler for backward compatibility
	err = csicontroller.NewReconciler(mgr.GetClient(), podConfig).SetupWithManager(mgr)
	if err != nil {
		log.Error(err, "failed to create pod reconciler")
		os.Exit(1)
	}

	// Setup the S3PodAttachment reconciler for v2 pod creation
	s3paReconciler := csicontroller.NewS3PodAttachmentReconciler(mgr.GetClient(), scheme, podConfig)
	if err := s3paReconciler.SetupWithManager(mgr); err != nil {
		log.Error(err, "failed to create S3PodAttachment reconciler")
		os.Exit(1)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "failed to start manager")
		os.Exit(1)
	}
}

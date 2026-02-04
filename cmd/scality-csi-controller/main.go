// `scality-csi-controller` is the entrypoint binary for the CSI Driver's controller component.
// It is responsible for acting on cluster events and spawning Mountpoint Pods when necessary.
// It manages Mountpoint Pods lifecycle and cleans up stale attachments.
package main

import (
	"flag"
	"os"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	mountpointNamespace                   = flag.String("mountpoint-namespace", os.Getenv("MOUNTPOINT_NAMESPACE"), "Namespace to spawn Mountpoint Pods in.")
	mountpointVersion                     = flag.String("mountpoint-version", os.Getenv("MOUNTPOINT_VERSION"), "Version of Mountpoint within the given Mountpoint image.")
	mountpointPriorityClassName           = flag.String("mountpoint-priority-class-name", os.Getenv("MOUNTPOINT_PRIORITY_CLASS_NAME"), "Priority class name of the Mountpoint Pods.")
	mountpointPreemptingPriorityClassName = flag.String("mountpoint-preempting-priority-class-name", os.Getenv("MOUNTPOINT_PREEMPTING_PRIORITY_CLASS_NAME"), "Preempting priority class name of the Mountpoint Pods.")
	mountpointHeadroomPriorityClassName   = flag.String("mountpoint-headroom-priority-class-name", os.Getenv("MOUNTPOINT_HEADROOM_PRIORITY_CLASS_NAME"), "Priority class name of the Headroom Pods.")
	mountpointImage                       = flag.String("mountpoint-image", os.Getenv("MOUNTPOINT_IMAGE"), "Image of Mountpoint to use in spawned Mountpoint Pods.")
	headroomImage                         = flag.String("headroom-image", os.Getenv("MOUNTPOINT_HEADROOM_IMAGE"), "Image of a pause container to use in spawned Headroom Pods.")
	mountpointImagePullPolicy             = flag.String("mountpoint-image-pull-policy", os.Getenv("MOUNTPOINT_IMAGE_PULL_POLICY"), "Pull policy of Mountpoint images.")
	mountpointContainerCommand            = flag.String("mountpoint-container-command", "/bin/scality-s3-csi-mounter", "Entrypoint command of the Mountpoint Pods.")

	// TLS configuration for custom CA certificates in mounter pods
	tlsCACertSecret           = flag.String("tls-ca-cert-secret", os.Getenv("TLS_CA_CERT_SECRET"), "Name of Kubernetes Secret containing custom CA certificate(s).")
	tlsInitImage              = flag.String("tls-init-image", os.Getenv("TLS_INIT_IMAGE"), "Image for CA certificate installation initContainer.")
	tlsInitImagePullPolicy    = flag.String("tls-init-image-pull-policy", os.Getenv("TLS_INIT_IMAGE_PULL_POLICY"), "Pull policy for TLS init image.")
	tlsInitResourcesReqCPU    = flag.String("tls-init-resources-req-cpu", os.Getenv("TLS_INIT_RESOURCES_REQUESTS_CPU"), "CPU request for TLS init container.")
	tlsInitResourcesReqMemory = flag.String("tls-init-resources-req-memory", os.Getenv("TLS_INIT_RESOURCES_REQUESTS_MEMORY"), "Memory request for TLS init container.")
	tlsInitResourcesLimMemory = flag.String("tls-init-resources-lim-memory", os.Getenv("TLS_INIT_RESOURCES_LIMITS_MEMORY"), "Memory limit for TLS init container.")
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(crdv2.AddToScheme(scheme))
}

// buildTLSConfig creates a TLSConfig from environment variables/flags.
// Returns nil if TLS is not configured (no CA cert secret specified).
func buildTLSConfig(log logr.Logger) *mppod.TLSConfig {
	if *tlsCACertSecret == "" {
		return nil
	}

	// Parse resource quantities with defaults
	reqCPU := resource.MustParse("10m")
	if *tlsInitResourcesReqCPU != "" {
		var err error
		reqCPU, err = resource.ParseQuantity(*tlsInitResourcesReqCPU)
		if err != nil {
			log.Error(err, "failed to parse TLS init CPU request, using default", "value", *tlsInitResourcesReqCPU)
			reqCPU = resource.MustParse("10m")
		}
	}

	reqMemory := resource.MustParse("16Mi")
	if *tlsInitResourcesReqMemory != "" {
		var err error
		reqMemory, err = resource.ParseQuantity(*tlsInitResourcesReqMemory)
		if err != nil {
			log.Error(err, "failed to parse TLS init memory request, using default", "value", *tlsInitResourcesReqMemory)
			reqMemory = resource.MustParse("16Mi")
		}
	}

	limMemory := resource.MustParse("64Mi")
	if *tlsInitResourcesLimMemory != "" {
		var err error
		limMemory, err = resource.ParseQuantity(*tlsInitResourcesLimMemory)
		if err != nil {
			log.Error(err, "failed to parse TLS init memory limit, using default", "value", *tlsInitResourcesLimMemory)
			limMemory = resource.MustParse("64Mi")
		}
	}

	// Default init image if not specified
	initImage := *tlsInitImage
	if initImage == "" {
		initImage = "alpine:3.21"
	}

	// Default pull policy if not specified
	initImagePullPolicy := corev1.PullPolicy(*tlsInitImagePullPolicy)
	if initImagePullPolicy == "" {
		initImagePullPolicy = corev1.PullIfNotPresent
	}

	log.Info("TLS configuration enabled for mounter pods",
		"caCertSecret", *tlsCACertSecret,
		"initImage", initImage)

	return &mppod.TLSConfig{
		CACertSecretName:       *tlsCACertSecret,
		InitImage:              initImage,
		InitImagePullPolicy:    initImagePullPolicy,
		InitResourcesReqCPU:    reqCPU,
		InitResourcesReqMemory: reqMemory,
		InitResourcesLimMemory: limMemory,
	}
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

	// Setup field indexers for MountpointS3PodAttachment CRDs
	if err := crdv2.SetupManagerIndices(mgr); err != nil {
		log.Error(err, "failed to setup field indexers")
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
		TLS:              buildTLSConfig(log),
	}

	// Setup the pod reconciler that will create MountpointS3PodAttachments
	reconciler := csicontroller.NewReconciler(mgr.GetClient(), podConfig)
	err = reconciler.SetupWithManager(mgr)
	if err != nil {
		log.Error(err, "failed to create pod reconciler")
		os.Exit(1)
	}

	// Setup signal handler once and share context
	ctx := signals.SetupSignalHandler()

	// Start stale attachment cleaner in background
	cleaner := csicontroller.NewStaleAttachmentCleaner(reconciler)
	go func() {
		if err := cleaner.Start(ctx); err != nil {
			log.Error(err, "stale attachment cleaner failed")
		}
	}()

	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "failed to start manager")
		os.Exit(1)
	}
}

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
	tlsCACertConfigMap                    = flag.String("tls-ca-cert-configmap", os.Getenv("TLS_CA_CERT_CONFIGMAP"), "Name of ConfigMap containing custom CA certificate(s).")
	tlsInitImage                          = flag.String("tls-init-image", os.Getenv("TLS_INIT_IMAGE"), "Image for CA certificate installation initContainer.")
	tlsInitImagePullPolicy                = flag.String("tls-init-image-pull-policy", os.Getenv("TLS_INIT_IMAGE_PULL_POLICY"), "Pull policy for TLS init image.")
	tlsInitResourcesReqCPU                = flag.String("tls-init-resources-req-cpu", os.Getenv("TLS_INIT_RESOURCES_REQUESTS_CPU"), "CPU request for TLS init container.")
	tlsInitResourcesReqMemory             = flag.String("tls-init-resources-req-memory", os.Getenv("TLS_INIT_RESOURCES_REQUESTS_MEMORY"), "Memory request for TLS init container.")
	tlsInitResourcesLimMemory             = flag.String("tls-init-resources-lim-memory", os.Getenv("TLS_INIT_RESOURCES_LIMITS_MEMORY"), "Memory limit for TLS init container.")
)

var scheme = runtime.NewScheme()

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

// buildTLSConfig constructs a TLSConfig from flags/env vars. Returns nil if no ConfigMap name is set.
func buildTLSConfig(log logr.Logger) *mppod.TLSConfig {
	if *tlsCACertConfigMap == "" {
		return nil
	}

	initImage := *tlsInitImage
	if initImage == "" {
		initImage = "alpine:3.21"
	}

	pullPolicy := corev1.PullPolicy(*tlsInitImagePullPolicy)
	if pullPolicy == "" {
		pullPolicy = corev1.PullIfNotPresent
	}

	reqCPU := resource.MustParse("10m")
	if *tlsInitResourcesReqCPU != "" {
		parsed, err := resource.ParseQuantity(*tlsInitResourcesReqCPU)
		if err != nil {
			log.Error(err, "invalid TLS init CPU request", "value", *tlsInitResourcesReqCPU)
			os.Exit(1)
		}
		reqCPU = parsed
	}

	reqMemory := resource.MustParse("16Mi")
	if *tlsInitResourcesReqMemory != "" {
		parsed, err := resource.ParseQuantity(*tlsInitResourcesReqMemory)
		if err != nil {
			log.Error(err, "invalid TLS init memory request", "value", *tlsInitResourcesReqMemory)
			os.Exit(1)
		}
		reqMemory = parsed
	}

	limMemory := resource.MustParse("64Mi")
	if *tlsInitResourcesLimMemory != "" {
		parsed, err := resource.ParseQuantity(*tlsInitResourcesLimMemory)
		if err != nil {
			log.Error(err, "invalid TLS init memory limit", "value", *tlsInitResourcesLimMemory)
			os.Exit(1)
		}
		limMemory = parsed
	}

	log.Info("TLS configuration enabled", "configmap", *tlsCACertConfigMap, "initImage", initImage)

	return &mppod.TLSConfig{
		CACertConfigMapName:    *tlsCACertConfigMap,
		InitImage:              initImage,
		InitImagePullPolicy:    pullPolicy,
		InitResourcesReqCPU:    reqCPU,
		InitResourcesReqMemory: reqMemory,
		InitResourcesLimMemory: limMemory,
	}
}

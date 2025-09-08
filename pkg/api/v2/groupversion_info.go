// Package v2 contains API Schema definitions for the s3.csi.scality.com v2 API group.
// +kubebuilder:object:generate=true
// +groupName=s3.csi.scality.com
package v2

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: constants.DriverName, Version: "v2"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

package ctrlclient

import (
	appv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	corev1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/core/v1alpha1"
	infastructurev1alpha3 "github.com/giantswarm/apiextensions/v3/pkg/apis/infrastructure/v1alpha3"
	releasev1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/release/v1alpha1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	expcapz "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	expcapi "sigs.k8s.io/cluster-api/exp/api/v1beta1"
)

var Scheme = runtime.NewScheme()

func init() {
	schemeBuilder := runtime.SchemeBuilder{
		apiextensions.AddToScheme,
		capi.AddToScheme,
		capz.AddToScheme,
		expcapi.AddToScheme,
		expcapz.AddToScheme,
		appsv1.AddToScheme,
		corev1.AddToScheme,
		corev1alpha1.AddToScheme,
		releasev1alpha1.AddToScheme,
		appv1alpha1.AddToScheme,
		infastructurev1alpha3.AddToScheme,
		networkingv1.AddToScheme,
		storagev1.AddToScheme,
		kyvernov2alpha1.AddToScheme,
	}
	err := schemeBuilder.AddToScheme(Scheme)
	if err != nil {
		panic(err)
	}
}

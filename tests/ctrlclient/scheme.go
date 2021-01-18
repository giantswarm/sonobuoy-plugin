package ctrlclient

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime"
	capzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	expcapzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	expcapiv1alpha3 "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
)

var Scheme = runtime.NewScheme()

func init() {
	schemeBuilder := runtime.SchemeBuilder{
		apiextensions.AddToScheme,
		capiv1alpha3.AddToScheme,
		capzv1alpha3.AddToScheme,
		expcapiv1alpha3.AddToScheme,
		expcapzv1alpha3.AddToScheme,
		appsv1.AddToScheme,
		corev1.AddToScheme,
	}
	err := schemeBuilder.AddToScheme(Scheme)
	if err != nil {
		panic(err)
	}
}

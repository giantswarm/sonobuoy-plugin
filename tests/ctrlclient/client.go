package ctrlclient

import (
	"context"
	"os"

	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	capzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	expcapzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	expcapiv1alpha3 "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func CreateCPCtrlClient(ctx context.Context) (client.Client, error) {
	kubeConfig, exists := os.LookupEnv("CP_KUBECONFIG")
	if !exists {
		return nil, microerror.Mask(missingEnvironmentVariable)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConfig))
	if err != nil {
		return nil, microerror.Mask(err)
	}

	runtimeScheme := runtime.NewScheme()
	appSchemeBuilder := runtime.SchemeBuilder{
		apiextensions.AddToScheme,
		capiv1alpha3.AddToScheme,
		capzv1alpha3.AddToScheme,
		expcapiv1alpha3.AddToScheme,
		expcapzv1alpha3.AddToScheme,
		corev1.AddToScheme,
	}
	err = appSchemeBuilder.AddToScheme(runtimeScheme)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return client.New(rest.CopyConfig(restConfig), client.Options{Scheme: runtimeScheme})
}

func CreateTCCtrlClient(ctx context.Context) (client.Client, error) {
	return client.New(config.GetConfigOrDie(), client.Options{})
}

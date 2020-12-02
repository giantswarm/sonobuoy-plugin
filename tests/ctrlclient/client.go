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
)

const (
	ControlPlaneKubeconfigContents  = "CP_KUBECONFIG"
	TenantClusterKubeconfigContents = "TC_KUBECONFIG"
)

func GetCPKubeConfig(ctx context.Context) ([]byte, error) {
	kubeConfig, exists := os.LookupEnv(ControlPlaneKubeconfigContents)
	if !exists {
		return nil, microerror.Maskf(missingEnvironmentVariable, "the %s env var is required", ControlPlaneKubeconfigContents)
	}

	return []byte(kubeConfig), nil
}

func GetTCKubeConfig(ctx context.Context) ([]byte, error) {
	kubeConfig, exists := os.LookupEnv(TenantClusterKubeconfigContents)
	if !exists {
		return nil, microerror.Maskf(missingEnvironmentVariable, "the %s env var is required", TenantClusterKubeconfigContents)
	}

	return []byte(kubeConfig), nil
}

func CreateTCCtrlClient(ctx context.Context) (client.Client, error) {
	kubeConfig, err := GetTCKubeConfig(ctx)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfig)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	scheme, err := getScheme()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return client.New(rest.CopyConfig(restConfig), client.Options{Scheme: scheme})
}

func CreateCPCtrlClient(ctx context.Context) (client.Client, error) {
	kubeConfig, err := GetCPKubeConfig(ctx)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConfig))
	if err != nil {
		return nil, microerror.Mask(err)
	}

	scheme, err := getScheme()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return client.New(restConfig, client.Options{Scheme: scheme})
}

func getScheme() (*runtime.Scheme, error) {
	runtimeScheme := runtime.NewScheme()
	appSchemeBuilder := runtime.SchemeBuilder{
		apiextensions.AddToScheme,
		capiv1alpha3.AddToScheme,
		capzv1alpha3.AddToScheme,
		expcapiv1alpha3.AddToScheme,
		expcapzv1alpha3.AddToScheme,
		corev1.AddToScheme,
	}
	err := appSchemeBuilder.AddToScheme(runtimeScheme)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return runtimeScheme, nil
}

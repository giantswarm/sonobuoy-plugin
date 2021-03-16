package ctrlclient

import (
	"os"

	"github.com/giantswarm/microerror"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ControlPlaneKubeconfigContents  = "CP_KUBECONFIG"
	TenantClusterKubeconfigContents = "TC_KUBECONFIG"
)

func GetCPKubeConfig() ([]byte, error) {
	kubeConfig, exists := os.LookupEnv(ControlPlaneKubeconfigContents)
	if !exists {
		return nil, microerror.Maskf(missingEnvironmentVariable, "the %s env var is required", ControlPlaneKubeconfigContents)
	}

	return []byte(kubeConfig), nil
}

func GetRestConfig() (*rest.Config, error) {
	kubeConfig, err := GetCPKubeConfig()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfig)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return restConfig, nil
}

func GetTCKubeConfig() ([]byte, error) {
	kubeConfig, exists := os.LookupEnv(TenantClusterKubeconfigContents)
	if !exists {
		return nil, microerror.Maskf(missingEnvironmentVariable, "the %s env var is required", TenantClusterKubeconfigContents)
	}

	return []byte(kubeConfig), nil
}

func CreateTCCtrlClient() (client.Client, error) {
	kubeConfig, err := GetTCKubeConfig()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfig)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return client.New(rest.CopyConfig(restConfig), client.Options{Scheme: Scheme})
}

func CreateCPCtrlClient() (client.Client, error) {
	restConfig, err := GetRestConfig()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return client.New(restConfig, client.Options{Scheme: Scheme})
}

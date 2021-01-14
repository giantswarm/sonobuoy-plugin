package ctrlclient

import (
	"context"
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

	return client.New(rest.CopyConfig(restConfig), client.Options{Scheme: Scheme})
}

func CreateCPCtrlClient(ctx context.Context) (client.Client, error) {
	kubeConfig, err := GetCPKubeConfig(ctx)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfig)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return client.New(restConfig, client.Options{Scheme: Scheme})
}

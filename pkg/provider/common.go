package provider

import (
	"context"
	"os"
	"strings"

	"github.com/giantswarm/microerror"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ProviderEnvVarName = "PROVIDER"
)

func GetProvider() string {
	provider := os.Getenv(ProviderEnvVarName)

	return strings.TrimSpace(provider)
}

func GetProviderSupport(ctx context.Context, client ctrl.Client, cluster *capi.Cluster) (Support, error) {
	switch GetProvider() {
	case "azure":
		p, err := NewAzureProviderSupport(ctx, client, cluster)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		return p, nil
	}

	return nil, microerror.Maskf(executionFailedError, "unsupported provider value in $%s: %q", ProviderEnvVarName, provider)
}

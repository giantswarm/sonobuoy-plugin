package provider

import (
	"context"
	"os"
	"strings"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ProviderEnvVarName = "PROVIDER"
)

func GetProvider() string {
	provider := os.Getenv(ProviderEnvVarName)

	return strings.TrimSpace(provider)
}

func GetProviderSupport(ctx context.Context, logger micrologger.Logger, client ctrl.Client, cluster *capi.Cluster) (Support, error) {
	switch GetProvider() {
	case "azure":
		p, err := NewAzureProviderSupport(ctx, logger, client, cluster)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		return p, nil
	case "aws":
		p, err := NewAWSProviderSupport(ctx, logger, client, cluster)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		return p, nil
	}

	return nil, microerror.Maskf(executionFailedError, "unsupported provider value in $%s: %q", ProviderEnvVarName, GetProvider())
}

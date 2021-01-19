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
	Provider = "E2E_PROVIDER"
)

func GetProviderSupport(ctx context.Context, client ctrl.Client, cluster *capi.Cluster) (Support, error) {
	provider := os.Getenv(Provider)

	switch strings.TrimSpace(provider) {
	case "azure":
		p, err := NewAzureProviderSupport(ctx, client, cluster)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		return p, nil
	}

	return nil, microerror.Maskf(executionFailedError, "unsupported provider value in $%s: %q", Provider, provider)
}

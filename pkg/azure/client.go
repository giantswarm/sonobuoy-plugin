package azure

import (
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/azure/credentials"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/azure/internal/client"
)

type ClientConfig struct {
	ServicePrincipal *credentials.ServicePrincipal
}

func NewClient(config ClientConfig) (*Client, error) {
	if config.ServicePrincipal == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.ServicePrincipal can't be nil")
	}

	sp := config.ServicePrincipal
	creds := auth.NewClientCredentialsConfig(sp.ClientID, sp.ClientSecret, sp.TenantID)
	authorizer, err := creds.Authorizer()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	c := &Client{
		ResourceGroup: client.NewGroupsClient(authorizer, sp.SubscriptionID),
	}

	return c, nil
}

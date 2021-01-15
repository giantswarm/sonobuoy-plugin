package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/azure/credentials"
)

type Config struct {
	ServicePrincipal *credentials.ServicePrincipal
}

type Client struct {
	groupsClient *resources.GroupsClient
}

func NewClient(config Config) (*Client, error) {
	if config.ServicePrincipal == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.ServicePrincipal can't be nil")
	}
	groupsClient, err := newResourceGroupClient(*config.ServicePrincipal)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return &Client{
		groupsClient: groupsClient,
	}, nil
}

func (c *Client) ResourceGroupExists(ctx context.Context, name string) (bool, error) {
	_, err := c.groupsClient.Get(ctx, name)
	if IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, microerror.Mask(err)
	}
	return true, nil
}

func newResourceGroupClient(servicePrincipal credentials.ServicePrincipal) (*resources.GroupsClient, error) {
	creds := auth.NewClientCredentialsConfig(servicePrincipal.ClientID, servicePrincipal.ClientSecret, servicePrincipal.TenantID)
	authorizer, err := creds.Authorizer()
	if err != nil {
		return nil, microerror.Mask(err)
	}
	client := resources.NewGroupsClient(servicePrincipal.SubscriptionID)
	client.Authorizer = authorizer
	return &client, nil
}

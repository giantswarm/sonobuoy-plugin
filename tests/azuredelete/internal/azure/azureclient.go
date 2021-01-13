package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/giantswarm/microerror"
	"os"
)

type Client struct {
	groupsClient *resources.GroupsClient
}

func NewClient() (*Client, error) {
	clientID, exists := os.LookupEnv("AZURE_WC_CLIENT_ID")
	if !exists {
		return nil, microerror.Maskf(missingEnvVarError, "missing AZURE_WC_CLIENT_ID environment variable")
	}
	clientSecret, exists := os.LookupEnv("AZURE_WC_CLIENT_SECRET")
	if !exists {
		return nil, microerror.Maskf(missingEnvVarError, "missing AZURE_WC_CLIENT_SECRET environment variable")
	}
	tenantID, exists := os.LookupEnv("AZURE_WC_TENANT_ID")
	if !exists {
		return nil, microerror.Maskf(missingEnvVarError, "missing AZURE_WC_TENANT_ID environment variable")
	}
	subscriptionID, exists := os.LookupEnv("AZURE_WC_SUBSCRIPTION_ID")
	if !exists {
		return nil, microerror.Maskf(missingEnvVarError, "missing AZURE_WC_SUBSCRIPTION_ID environment variable")
	}
	groupsClient, err := newResourceGroupClient(clientID, clientSecret, tenantID, subscriptionID)
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

func newResourceGroupClient(clientID, clientSecret, tenantID, subscriptionID string) (*resources.GroupsClient, error) {
	credentials := auth.NewClientCredentialsConfig(clientID, clientSecret, tenantID)
	authorizer, err := credentials.Authorizer()
	if err != nil {
		return nil, microerror.Mask(err)
	}
	client := resources.NewGroupsClient(subscriptionID)
	client.Authorizer = authorizer
	return &client, nil
}

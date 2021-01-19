package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/giantswarm/microerror"
)

// GroupsClient wraps an Azure SDK GroupsClient.
type GroupsClient struct {
	resources.GroupsClient
}

func NewGroupsClient(authorizer autorest.Authorizer, subscriptionID string) *GroupsClient {
	client := resources.NewGroupsClient(subscriptionID)
	client.Authorizer = authorizer
	return &GroupsClient{
		GroupsClient: client,
	}
}

func (c *GroupsClient) Exists(ctx context.Context, name string) (bool, error) {
	_, err := c.Get(ctx, name)
	if IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, microerror.Mask(err)
	}
	return true, nil
}

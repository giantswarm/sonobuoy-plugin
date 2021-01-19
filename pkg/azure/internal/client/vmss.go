package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/go-autorest/autorest"
)

// Type wrapper
type VMSS *compute.VirtualMachineScaleSet

// VMSSClient wraps an Azure SDK VirtualMachineScaleSetsClient.
type VMSSClient struct {
	compute.VirtualMachineScaleSetsClient
}

func NewVMSSClient(authorizer autorest.Authorizer, subscriptionID string) *VMSSClient {
	client := compute.NewVirtualMachineScaleSetsClient(subscriptionID)
	client.Authorizer = authorizer

	return &VMSSClient{
		VirtualMachineScaleSetsClient: client,
	}
}

func (c *VMSSClient) Get(ctx context.Context, resourceGroupName, vmssName string) (VMSS, error) {
	return c.Get(ctx, resourceGroupName, vmssName)
}

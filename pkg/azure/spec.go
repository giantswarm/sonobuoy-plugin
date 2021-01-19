package azure

import "context"

type ResourceGroupsClient interface {
	Exists(ctx context.Context, name string) (bool, error)
}

type VMSSClient interface {
	Get(ctx context.Context, resourceGroupName, vmssName string) (VMSS, error)
}

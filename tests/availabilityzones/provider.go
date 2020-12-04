package availabilityzones

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/giantswarm/microerror"
)

type AzureNodePoolAZsGetter struct {
	virtualMachineScaleSetsClient *compute.VirtualMachineScaleSetsClient
}

func NewAzureNodePoolAZsGetter(virtualMachineScaleSetsClient *compute.VirtualMachineScaleSetsClient) (NodePoolAZGetter, error) {
	if virtualMachineScaleSetsClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "virtualMachineScaleSetsClient must not be empty")
	}

	p := &AzureNodePoolAZsGetter{
		virtualMachineScaleSetsClient: virtualMachineScaleSetsClient,
	}

	return p, nil
}

func (p *AzureNodePoolAZsGetter) GetNodePoolAZs(ctx context.Context, clusterID, nodepoolName string) ([]string, error) {
	var zones []string
	nodepoolVMSSName := fmt.Sprintf("nodepool-%s", nodepoolName)
	vmss, err := p.virtualMachineScaleSetsClient.Get(ctx, clusterID, nodepoolVMSSName)
	if err != nil {
		return zones, microerror.Mask(err)
	}

	if vmss.Zones != nil {
		return *vmss.Zones, nil
	}

	return zones, nil
}

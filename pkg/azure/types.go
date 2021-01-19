package azure

import "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"

// Client groups different Azure API clients together as a convenient facade.
type Client struct {
	ResourceGroup ResourceGroupsClient
	VMSS          VMSSClient
}

/*
 * Azure SDK Type wrappers
 */

type VMSS *compute.VirtualMachineScaleSet

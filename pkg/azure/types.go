package azure

import "github.com/giantswarm/sonobuoy-plugin/v5/pkg/azure/internal/client"

// Client groups different Azure API clients together as a convenient facade.
type Client struct {
	ResourceGroup ResourceGroupsClient
	VMSS          VMSSClient
}

/*
 * Azure SDK Type wrappers
 */

type VMSS = client.VMSS

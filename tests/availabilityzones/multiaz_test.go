package availabilityzones

import (
	"context"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/giantswarm/microerror"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/tests/ctrlclient"
)

const (
	Provider = "E2E_PROVIDER"
)

func Test_AvailabilityZones(t *testing.T) {
	var err error

	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	machinePoolList := &v1alpha3.MachinePoolList{}
	err = cpCtrlClient.List(ctx, machinePoolList, client.MatchingLabels{capiv1alpha3.ClusterLabelName: clusterID})
	if err != nil {
		t.Fatal(err)
	}

	expectedZones := machinePoolList.Items[0].Spec.FailureDomains

	nodePoolAZsGetter, err := getNodepoolAZsGetter()
	if err != nil {
		t.Fatal(err)
	}

	actualZones, err := nodePoolAZsGetter.GetNodePoolAZs(ctx, clusterID, machinePoolList.Items[0].Name)
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(actualZones)
	sort.Strings(expectedZones)

	if !reflect.DeepEqual(actualZones, expectedZones) {
		t.Fatalf("The AZs used are not correct. Expected %s, got %s", expectedZones, actualZones)
	}
}

func getNodepoolAZsGetter() (NodePoolAZGetter, error) {
	var nodePoolAZsGetter NodePoolAZGetter
	if os.Getenv(Provider) == "azure" {
		virtualMachineScaleSetsClient, err := getVirtualMachineScaleSetsClient()
		if err != nil {
			return nil, err
		}

		nodePoolAZsGetter, err = NewAzureNodePoolAZsGetter(&virtualMachineScaleSetsClient)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, microerror.Maskf(invalidConfigError, "chosen provider %s must be one of: azure", Provider)
	}

	return nodePoolAZsGetter, nil
}

func getVirtualMachineScaleSetsClient() (compute.VirtualMachineScaleSetsClient, error) {
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return compute.VirtualMachineScaleSetsClient{}, microerror.Mask(err)
	}

	authorizer, err := settings.GetAuthorizer()
	if err != nil {
		return compute.VirtualMachineScaleSetsClient{}, microerror.Mask(err)
	}

	virtualMachineScaleSetsClient := compute.NewVirtualMachineScaleSetsClient(settings.GetSubscriptionID())
	virtualMachineScaleSetsClient.Client.Authorizer = authorizer

	return virtualMachineScaleSetsClient, nil
}

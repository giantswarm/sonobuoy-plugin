package availabilityzones

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	expcapz "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	expcapi "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Provider = "E2E_PROVIDER"

	defaultVMSize = "Standard_D4s_v3"

	clientIDKey       = "azure.azureoperator.clientid"
	clientSecretKey   = "azure.azureoperator.clientsecret"
	defaultAzureGUID  = "37f13270-5c7a-56ff-9211-8426baaeaabd"
	partnerIDKey      = "azure.azureoperator.partnerid"
	subscriptionIDKey = "azure.azureoperator.subscriptionid"
	tenantIDKey       = "azure.azureoperator.tenantid"
)

type AzureProviderSupport struct {
	virtualMachineScaleSetsClient *compute.VirtualMachineScaleSetsClient
}

func GetProviderSupport(ctx context.Context, client ctrl.Client, cluster *capi.Cluster) (ProviderSupport, error) {
	switch os.Getenv(Provider) {
	case "azure":
		p, err := NewAzureProviderSupport(ctx, client, cluster)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		return p, nil
	}

	return nil, microerror.Maskf(executionFailedError, "unsupported provider")
}

func NewAzureProviderSupport(ctx context.Context, client ctrl.Client, cluster *capi.Cluster) (ProviderSupport, error) {
	credentials, subscriptionID, err := findAzureCredentials(ctx, client, cluster)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	vmssClient, err := createVMSSClient(credentials, subscriptionID)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	p := &AzureProviderSupport{
		virtualMachineScaleSetsClient: vmssClient,
	}

	return p, nil
}

func (p *AzureProviderSupport) CreateNodePool(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azs []string) (*expcapi.MachinePool, error) {
	azureMP, err := p.createAzureMachinePool(ctx, client, cluster, azs)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	mp, err := p.createMachinePool(ctx, client, cluster, azureMP, azs)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return mp, nil
}

func (p *AzureProviderSupport) GetProviderAZs() []string {
	return []string{"1", "2", "3"}
}

func (p *AzureProviderSupport) GetNodePoolAZs(ctx context.Context, clusterID, nodepoolID string) ([]string, error) {
	var zones []string
	nodepoolVMSSName := fmt.Sprintf("nodepool-%s", nodepoolID)
	vmss, err := p.virtualMachineScaleSetsClient.Get(ctx, clusterID, nodepoolVMSSName)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if vmss.Zones != nil {
		zones = *vmss.Zones
	}

	return zones, nil
}

func (p *AzureProviderSupport) createMachinePool(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azureMachinePool *expcapz.AzureMachinePool, azs []string) (*expcapi.MachinePool, error) {
	var infrastructureCRRef *corev1.ObjectReference
	{
		s := runtime.NewScheme()
		err := expcapz.AddToScheme(s)
		if err != nil {
			panic(fmt.Sprintf("expcapz.AddToScheme: %+v", err))
		}

		infrastructureCRRef, err = reference.GetReference(s, azureMachinePool)
		if err != nil {
			panic(fmt.Sprintf("cannot create reference to infrastructure CR: %q", err))
		}
	}

	machinePool := &expcapi.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      azureMachinePool.Name,
			Namespace: azureMachinePool.Namespace,
			Labels: map[string]string{
				label.AzureOperatorVersion: cluster.Labels[label.AzureOperatorVersion],
				label.Cluster:              cluster.Labels[label.Cluster],
				capi.ClusterLabelName:      cluster.Labels[capi.ClusterLabelName],
				label.MachinePool:          azureMachinePool.Labels[label.MachinePool],
				label.Organization:         cluster.Labels[label.Organization],
				label.ReleaseVersion:       cluster.Labels[label.ReleaseVersion],
			},
			Annotations: map[string]string{
				annotation.MachinePoolName: "availability zone verification e2e test",
				annotation.NodePoolMinSize: fmt.Sprintf("%d", len(azs)),
				annotation.NodePoolMaxSize: fmt.Sprintf("%d", len(azs)),
			},
		},
		Spec: expcapi.MachinePoolSpec{
			ClusterName:    cluster.Name,
			Replicas:       to.Int32Ptr(int32(len(azs))),
			FailureDomains: azs,
			Template: capi.MachineTemplateSpec{
				Spec: capi.MachineSpec{
					ClusterName:       cluster.Name,
					InfrastructureRef: *infrastructureCRRef,
				},
			},
		},
	}

	err := client.Create(ctx, machinePool)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return machinePool, nil
}

func (p *AzureProviderSupport) createAzureMachinePool(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azs []string) (*expcapz.AzureMachinePool, error) {
	azureCluster := &capz.AzureCluster{}
	{
		n := cluster.Spec.InfrastructureRef.Name
		ns := cluster.Spec.InfrastructureRef.Namespace
		err := client.Get(ctx, ctrl.ObjectKey{Name: n, Namespace: ns}, azureCluster)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	azureMachinePool := &expcapz.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2etst",
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				label.AzureOperatorVersion: cluster.Labels[label.AzureOperatorVersion],
				label.Cluster:              cluster.Labels[label.Cluster],
				capi.ClusterLabelName:      cluster.Name,
				label.MachinePool:          "e2etst",
				label.Organization:         cluster.Labels[label.Organization],
				label.ReleaseVersion:       cluster.Labels[label.ReleaseVersion],
			},
		},
		Spec: expcapz.AzureMachinePoolSpec{
			Location: azureCluster.Spec.Location,
			Template: expcapz.AzureMachineTemplate{
				DataDisks: []capz.DataDisk{
					{
						NameSuffix: "docker",
						DiskSizeGB: int32(100),
						Lun:        to.Int32Ptr(21),
					},
					{
						NameSuffix: "kubelet",
						DiskSizeGB: int32(100),
						Lun:        to.Int32Ptr(22),
					},
				},
				VMSize: defaultVMSize,
			},
		},
	}

	err := client.Create(ctx, azureMachinePool)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return azureMachinePool, nil
}

func findAzureCredentials(ctx context.Context, client ctrl.Client, cluster *capi.Cluster) (auth.ClientCredentialsConfig, string, error) {
	var secret *corev1.Secret
	var err error
	{
		var secretList corev1.SecretList
		err = client.List(ctx, &secretList, ctrl.MatchingLabels{label.Organization: cluster.Labels[label.Organization]})
		if err != nil {
			return auth.ClientCredentialsConfig{}, "", microerror.Mask(err)
		}

		if len(secretList.Items) > 0 {
			secret = &secretList.Items[0]
		} else {
			secret := &corev1.Secret{}
			err = client.Get(ctx, ctrl.ObjectKey{Name: "credential-default", Namespace: "giantswarm"}, secret)
			if err != nil {
				return auth.ClientCredentialsConfig{}, "", microerror.Mask(err)
			}
		}
	}

	var credentials auth.ClientCredentialsConfig
	var subscriptionID string
	{
		credentials.ClientID, err = valueFromSecret(secret, clientIDKey)
		if err != nil {
			return auth.ClientCredentialsConfig{}, "", microerror.Mask(err)
		}

		credentials.ClientSecret, err = valueFromSecret(secret, clientSecretKey)
		if err != nil {
			return auth.ClientCredentialsConfig{}, "", microerror.Mask(err)
		}

		credentials.TenantID, err = valueFromSecret(secret, tenantIDKey)
		if err != nil {
			return auth.ClientCredentialsConfig{}, "", microerror.Mask(err)
		}

		subscriptionID, err = valueFromSecret(secret, subscriptionIDKey)
		if err != nil {
			return auth.ClientCredentialsConfig{}, "", microerror.Mask(err)
		}
	}

	return credentials, subscriptionID, nil
}

func createVMSSClient(credentials auth.ClientCredentialsConfig, subscriptionID string) (*compute.VirtualMachineScaleSetsClient, error) {
	authorizer, err := credentials.Authorizer()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	virtualMachineScaleSetsClient := compute.NewVirtualMachineScaleSetsClient(subscriptionID)
	virtualMachineScaleSetsClient.Client.Authorizer = authorizer

	return &virtualMachineScaleSetsClient, nil
}

func valueFromSecret(secret *corev1.Secret, key string) (string, error) {
	v, ok := secret.Data[key]
	if !ok {
		return "", microerror.Maskf(missingValueError, key)
	}

	return string(v), nil
}

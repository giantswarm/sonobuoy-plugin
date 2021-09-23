package provider

import (
	"context"
	"fmt"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/microerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/reference"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	expcapz "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha4"
	expcapi "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/azure"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/azure/credentials"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/randomid"
)

const (
	defaultVMSize = "Standard_D4s_v3"
)

type AzureProviderSupport struct {
	azureClient *azure.Client
}

func NewAzureProviderSupport(ctx context.Context, client ctrl.Client, cluster *capi.Cluster) (Support, error) {
	sp, err := credentials.ForCluster(ctx, client, cluster)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	azureClient, err := azure.NewClient(azure.ClientConfig{ServicePrincipal: sp})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	p := &AzureProviderSupport{
		azureClient: azureClient,
	}

	return p, nil
}

func (p *AzureProviderSupport) CreateNodePool(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azs []string) (*expcapi.MachinePool, error) {
	azureMP, err := p.createAzureMachinePool(ctx, client, cluster, azs)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	bootstrap, err := p.createKubeadmConfig(ctx, client, cluster, azureMP.Name)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	mp, err := p.createMachinePool(ctx, client, cluster, azureMP, bootstrap, azs)
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
	vmss, err := p.azureClient.VMSS.Get(ctx, clusterID, nodepoolID)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if vmss.Zones != nil {
		zones = *vmss.Zones
	}

	return zones, nil
}

func (p *AzureProviderSupport) createMachinePool(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azureMachinePool *expcapz.AzureMachinePool, bootstrapCR *v1alpha4.KubeadmConfig, azs []string) (*expcapi.MachinePool, error) {
	infrastructureCRRef, err := reference.GetReference(client.Scheme(), azureMachinePool)
	if err != nil {
		panic(fmt.Sprintf("cannot create reference to infrastructure CR: %q", err))
	}

	bootstrapCRRef, err := reference.GetReference(client.Scheme(), bootstrapCR)
	if err != nil {
		panic(fmt.Sprintf("cannot create reference to bootstrap CR: %q", err))
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
				capiutil.E2ENodepool:       "true",
				capi.WatchLabel:            "capi",
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
					Bootstrap: capi.Bootstrap{
						ConfigRef: bootstrapCRRef,
					},
					ClusterName:       cluster.Name,
					InfrastructureRef: *infrastructureCRRef,
					Version:           to.StringPtr("v1.19.9"),
				},
			},
		},
	}

	err = client.Create(ctx, machinePool)
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

	nodepoolName := randomid.New()

	azureMachinePool := &expcapz.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodepoolName,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				label.AzureOperatorVersion: cluster.Labels[label.AzureOperatorVersion],
				label.Cluster:              cluster.Labels[label.Cluster],
				capi.ClusterLabelName:      cluster.Name,
				label.MachinePool:          nodepoolName,
				label.Organization:         cluster.Labels[label.Organization],
				label.ReleaseVersion:       cluster.Labels[label.ReleaseVersion],
				capiutil.E2ENodepool:       "true",
				capi.WatchLabel:            "capi",
			},
		},
		Spec: expcapz.AzureMachinePoolSpec{
			AdditionalTags: map[string]string{
				"cluster-autoscaler-enabled": "true",
				"cluster-autoscaler-name":    "u7kt3",
				"min":                        "3",
				"max":                        "10",
			},
			Identity: capz.VMIdentitySystemAssigned,
			Location: azureCluster.Spec.Location,
			Template: expcapz.AzureMachinePoolMachineTemplate{
				OSDisk: capz.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: to.Int32Ptr(30),
					ManagedDisk: &capz.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
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

func (p *AzureProviderSupport) createKubeadmConfig(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, nodepoolName string) (*v1alpha4.KubeadmConfig, error) {
	kubeadmConfig := &v1alpha4.KubeadmConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodepoolName,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				label.Cluster:         cluster.Name,
				capi.ClusterLabelName: cluster.Name,
				label.Organization:    cluster.Labels[label.Organization],
				label.ReleaseVersion:  cluster.Labels[label.ReleaseVersion],
				capiutil.E2ENodepool:  "true",
				capi.WatchLabel:       "capi",
			},
		},
		Spec: v1alpha4.KubeadmConfigSpec{
			JoinConfiguration: &v1alpha4.JoinConfiguration{
				NodeRegistration: v1alpha4.NodeRegistrationOptions{
					Name: "{{ ds.meta_data[\"local_hostname\"] }}",
					KubeletExtraArgs: map[string]string{
						"azure-container-registry-config": "/etc/kubernetes/azure.json",
						"cloud-config":                    "/etc/kubernetes/azure.json",
						"cloud-provider":                  "azure",
					},
				},
			},
			Files: []v1alpha4.File{
				{
					Path:        "/etc/kubernetes/azure.json",
					Owner:       "root:root",
					Permissions: "0644",
					ContentFrom: &v1alpha4.FileSource{Secret: v1alpha4.SecretFileSource{
						Name: fmt.Sprintf("%s-azure-json", nodepoolName),
						Key:  "worker-node-azure.json",
					}},
				},
			},
		},
	}

	err := client.Create(ctx, kubeadmConfig)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return kubeadmConfig, nil
}

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	corev1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	expcapz "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	expcapi "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
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
	logger      micrologger.Logger
	azureClient *azure.Client
}

func NewAzureProviderSupport(ctx context.Context, logger micrologger.Logger, client ctrl.Client, cluster *capi.Cluster) (Support, error) {
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
		logger:      logger,
	}

	return p, nil
}

func (p *AzureProviderSupport) CreateNodePoolAndWaitReady(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azs []string) (*ctrl.ObjectKey, error) {
	azureMP, err := p.createAzureMachinePool(ctx, client, cluster, azs)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	bootstrap, err := p.createSpark(ctx, client, cluster, azureMP)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	mp, err := p.createMachinePool(ctx, client, cluster, azureMP, bootstrap, azs)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	// Wait for Node Pool to come up.
	{
		o := func() error {
			err := client.Get(ctx, ctrl.ObjectKey{Name: mp.Name, Namespace: mp.Namespace}, mp)
			if err != nil {
				// Wrap masked error with backoff.Permanent() to stop retries on unrecoverable error.
				return backoff.Permanent(microerror.Mask(err))
			}

			// Return error for retry until node pool nodes are Ready.
			if !capiconditions.IsTrue(mp, capi.ReadyCondition) {
				return errors.New("node pool is not ready yet")
			}

			return nil
		}
		b := backoff.NewConstant(backoff.LongMaxWait, backoff.LongMaxInterval)
		n := backoff.NewNotifier(p.logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			return nil, microerror.Mask(fmt.Errorf("failed to get MachinePool %q for Cluster %q: %s", mp.Name, cluster.Name, microerror.JSON(err)))
		}
	}

	return &ctrl.ObjectKey{Name: mp.Name, Namespace: mp.Namespace}, nil
}

func (p *AzureProviderSupport) DeleteNodePool(ctx context.Context, client ctrl.Client, objKey ctrl.ObjectKey) error {
	mp := &expcapi.MachinePool{}

	err := client.Get(ctx, objKey, mp)
	if err != nil {
		return microerror.Mask(err)
	}

	return client.Delete(ctx, mp)
}

func (p *AzureProviderSupport) GetNodeSelectorLabel() string {
	return "giantswarm.io/machine-pool"
}

func (p *AzureProviderSupport) GetTestingMachinePoolForCluster(ctx context.Context, client ctrl.Client, clusterID string) (string, error) {
	var machinePoolName string
	{
		machinePools, err := capiutil.FindNonTestingMachinePoolsForCluster(ctx, client, clusterID)
		if err != nil {
			return "", fmt.Errorf("error finding MachinePools for cluster %q: %s", clusterID, microerror.JSON(err))
		}

		if len(machinePools) == 0 {
			return "", fmt.Errorf("expected one machine pool to exist, none found")
		}

		machinePoolName = machinePools[0].Name
	}

	return machinePoolName, nil
}

func (p *AzureProviderSupport) GetProviderAZs() []string {
	return []string{"1", "2", "3"}
}

func (p *AzureProviderSupport) GetNodePoolAZsInCR(ctx context.Context, client ctrl.Client, objKey ctrl.ObjectKey) ([]string, error) {
	mp := &expcapi.MachinePool{}

	err := client.Get(ctx, objKey, mp)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return mp.Spec.FailureDomains, nil
}

func (p *AzureProviderSupport) GetNodePoolAZsInProvider(ctx context.Context, clusterID, nodepoolID string) ([]string, error) {
	var zones []string
	nodepoolVMSSName := fmt.Sprintf("nodepool-%s", nodepoolID)
	vmss, err := p.azureClient.VMSS.Get(ctx, clusterID, nodepoolVMSSName)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if vmss.Zones != nil {
		zones = *vmss.Zones
	}

	return zones, nil
}

func (p *AzureProviderSupport) createMachinePool(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azureMachinePool *expcapz.AzureMachinePool, spark *corev1alpha1.Spark, azs []string) (*expcapi.MachinePool, error) {
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

	var bootstrapCRRef *corev1.ObjectReference
	{
		s := runtime.NewScheme()
		err := corev1alpha1.AddToScheme(s)
		if err != nil {
			panic(fmt.Sprintf("corev1alpha.AddToScheme: %+v", err))
		}

		bootstrapCRRef, err = reference.GetReference(s, spark)
		if err != nil {
			panic(fmt.Sprintf("cannot create reference to bootstrap CR: %q", err))
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
				capiutil.E2ENodepool:       "true",
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

func (p *AzureProviderSupport) createSpark(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azureMachinePool *expcapz.AzureMachinePool) (*corev1alpha1.Spark, error) {
	spark := &corev1alpha1.Spark{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Spark",
			APIVersion: "core.giantswarm.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      azureMachinePool.Name,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				label.Cluster:         cluster.Labels[label.Cluster],
				label.ReleaseVersion:  cluster.Labels[label.ReleaseVersion],
				capi.ClusterLabelName: cluster.Name,
				capiutil.E2ENodepool:  "true",
			},
		},
		Spec: corev1alpha1.SparkSpec{},
	}

	err := client.Create(ctx, spark)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return spark, nil
}

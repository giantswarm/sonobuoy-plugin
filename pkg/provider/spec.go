package provider

import (
	"context"

	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

type Support interface {
	CreateNodePoolAndWaitReady(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azs []string) (*ctrl.ObjectKey, error)
	DeleteNodePool(ctx context.Context, client ctrl.Client, objKey ctrl.ObjectKey) error
	GetNodeSelectorLabel() string
	GetTestingMachinePoolForCluster(ctx context.Context, client ctrl.Client, clusterID string) (string, error)
	GetProviderAZs() []string
	GetNodePoolAZsInCR(ctx context.Context, client ctrl.Client, objKey ctrl.ObjectKey) ([]string, error)
	GetNodePoolAZsInProvider(ctx context.Context, clusterID, nodepoolName string) ([]string, error)
}

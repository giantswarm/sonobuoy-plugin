package provider

import (
	"context"

	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	expcapi "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

type Support interface {
	CreateNodePool(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azs []string) (*expcapi.MachinePool, error)
	GetProviderAZs() []string
	GetNodePoolAZs(ctx context.Context, clusterID, nodepoolName string) ([]string, error)
}

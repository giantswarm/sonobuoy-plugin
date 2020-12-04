package availabilityzones

import (
	"context"
)

type NodePoolAZGetter interface {
	GetNodePoolAZs(ctx context.Context, clusterID, nodepoolName string) ([]string, error)
}

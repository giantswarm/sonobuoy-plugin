package sonobuoy_plugin

import (
	"context"
	"os"
	"testing"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/conditions/pkg/conditions"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/assert"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

func Test_ClusterCR(t *testing.T) {
	t.Parallel()

	var err error
	ctx := context.Background()

	regularLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := NewTestLogger(regularLogger, t)

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient()
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	clusterGetter := func(clusterName string) capiutil.TestedObject {
		cluster, err := capiutil.FindCluster(ctx, cpCtrlClient, clusterName)
		if err != nil {
			t.Fatalf("error finding cluster: %s", microerror.JSON(err))
		}

		return cluster
	}

	cluster := clusterGetter(clusterID).(*capi.Cluster)

	//
	// Test metadata
	//

	// Check if 'release.giantswarm.io/version' label is set
	assert.LabelIsSet(t, cluster, label.ReleaseVersion)

	// Wait for Ready condition to be True
	capiutil.WaitForCondition(t, ctx, logger, cluster, capi.ReadyCondition, capiconditions.IsTrue, clusterGetter)

	//
	// Test Status
	//

	if !cluster.Status.ControlPlaneReady {
		t.Fatalf("control plane is not ready")
	}

	if !cluster.Status.InfrastructureReady {
		t.Fatalf("infrastructure is not ready")
	}

	// Verify that ControlPlaneReady condition is True
	if !conditions.IsControlPlaneReadyTrue(cluster) {
		t.Fatalf("Cluster ControlPlaneReady condition is not True")
	}

	// Verify that InfrastructureReady condition is True
	if !conditions.IsInfrastructureReadyTrue(cluster) {
		t.Fatalf("Cluster InfrastructureReady condition is not True")
	}
}

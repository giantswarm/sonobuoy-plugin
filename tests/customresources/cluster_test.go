package customresources

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/conditions/pkg/conditions"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

type conditionCheck func(cluster capiconditions.Getter, conditionType capi.ConditionType) bool

func Test_ClusterCR(t *testing.T) {
	var err error
	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	clusterGetter := func() *capi.Cluster {
		return getTestedWorkloadCluster(ctx, t, cpCtrlClient)
	}

	cluster := clusterGetter()

	//
	// Test metadata
	//
	desiredRelease := cluster.Labels[label.ReleaseVersion]
	lastDeployedReleaseRelease := cluster.Annotations[annotation.LastDeployedReleaseVersion]
	if lastDeployedReleaseRelease != desiredRelease {
		t.Fatalf(
			"expected last deployed release %q and desired release %q to match",
			lastDeployedReleaseRelease,
			desiredRelease)
	}

	//
	// Test Status
	//
	if !cluster.Status.ControlPlaneInitialized {
		t.Fatalf("control plane is not initialized")
	}

	if !cluster.Status.ControlPlaneReady {
		t.Fatalf("control plane is not ready")
	}

	if !cluster.Status.InfrastructureReady {
		t.Fatalf("infrastructure is not ready")
	}

	// Wait for Ready condition to be True
	waitForClusterCondition(cluster, capi.ReadyCondition, capiconditions.IsTrue, clusterGetter)

	// Wait for Creating condition to be False
	waitForClusterCondition(cluster, conditions.Creating, capiconditions.IsFalse, clusterGetter)

	// Wait for Upgrading condition to be False
	waitForClusterCondition(cluster, conditions.Upgrading, capiconditions.IsFalse, clusterGetter)

	// Verify that Creating condition has Reason=CreationCompleted
	if !conditions.IsCreatingFalse(cluster, conditions.WithCreationCompletedReason()) {
		creatingCondition, _ := conditions.GetCreating(cluster)
		t.Fatalf(
			"Cluster Creating condition have unexpected Reason, expected 'CreationCompleted', got '%s'",
			creatingCondition.Reason)
	}

	// Verify that ControlPlaneReady condition is True
	if !conditions.IsControlPlaneReadyTrue(cluster) {
		t.Fatalf("Cluster ControlPlaneReady condition is not True")
	}

	// Verify that InfrastructureReady condition is True
	if !conditions.IsInfrastructureReadyTrue(cluster) {
		t.Fatalf("Cluster InfrastructureReady condition is not True")
	}

	// Verify that NodePoolsReady condition is True
	if !conditions.IsNodePoolsReadyTrue(cluster) {
		t.Fatalf("Cluster NodePoolsReady condition is not True")
	}
}

func waitForClusterCondition(cluster *capi.Cluster, conditionType capi.ConditionType, check conditionCheck, clusterGetterFunc clusterGetterFunc) {
	checkResult := check(cluster, conditionType)

	for ; checkResult != true; checkResult = check(cluster, conditionType) {
		time.Sleep(1 * time.Minute)
		updatedClusterCR := clusterGetterFunc()
		*cluster = *updatedClusterCR
	}
}

type clusterGetterFunc func() *capi.Cluster

func getTestedWorkloadCluster(ctx context.Context, t *testing.T, cpCtrlClient client.Client) *capi.Cluster {
	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	clusterList := &capi.ClusterList{}
	err := cpCtrlClient.List(ctx, clusterList, client.MatchingLabels{capi.ClusterLabelName: clusterID})
	if err != nil {
		t.Fatalf("error listing Clusters in CP k8s API: %v", err)
	}

	if len(clusterList.Items) != 1 {
		t.Fatalf("found %d clusters with cluster ID %s", len(clusterList.Items), clusterID)
	}

	cluster := clusterList.Items[0]
	return &cluster
}

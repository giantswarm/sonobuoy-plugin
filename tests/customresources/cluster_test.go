package customresources

import (
	"context"
	"testing"
	"time"

	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/conditions/pkg/conditions"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

func Test_ClusterCR(t *testing.T) {
	var err error
	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	clusterGetter := func() *capi.Cluster {
		return getTestedCluster(ctx, t, cpCtrlClient)
	}

	cluster := clusterGetter()

	//
	// Test metadata
	//

	// Check if 'release.giantswarm.io/version' label is set
	assertLabelIsSet(t, cluster, label.ReleaseVersion)

	// Check if 'release.giantswarm.io/last-deployed-version' annotation is set
	assertAnnotationIsSet(t, cluster, annotation.LastDeployedReleaseVersion)

	// Check if 'azure-operator.giantswarm.io/version' label is set
	assertLabelIsSet(t, cluster, label.AzureOperatorVersion)

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

package sonobuoy_plugin

import (
	"context"
	"os"
	"testing"

	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/conditions/pkg/conditions"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/assert"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/key"
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
	// This test only applies to GS clusters.
	release := cluster.Labels[label.ReleaseVersion]
	if key.IsCapiRelease(release) {
		logger.LogCtx(ctx, "level", "info", "message", "Test_ClusterCR in not used for CAPZ clusters")
		return
	}

	//
	// Test metadata
	//

	// Check if 'release.giantswarm.io/version' label is set
	assert.LabelIsSet(t, cluster, label.ReleaseVersion)

	// Check if 'azure-operator.giantswarm.io/version' label is set
	assert.LabelIsSet(t, cluster, label.AzureOperatorVersion)

	//
	// Wait for main conditions checking the remaining parts of the resource:
	//   Ready     == True
	//   Creating  == False
	//   Upgrading == False
	//

	// Wait for Ready condition to be True
	capiutil.WaitForCondition(t, ctx, logger, cluster, capi.ReadyCondition, capiconditions.IsTrue, clusterGetter)

	// Wait for Creating condition to be False
	capiutil.WaitForCondition(t, ctx, logger, cluster, conditions.Creating, capiconditions.IsFalse, clusterGetter)

	// Wait for Upgrading condition to be False
	capiutil.WaitForCondition(t, ctx, logger, cluster, conditions.Upgrading, capiconditions.IsFalse, clusterGetter)

	//
	// Continue checking metadata
	//

	// Check if 'release.giantswarm.io/last-deployed-version' annotation is set
	assert.AnnotationIsSet(t, cluster, annotation.LastDeployedReleaseVersion)

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

	if !cluster.Status.ControlPlaneReady {
		t.Fatalf("control plane is not ready")
	}

	if !cluster.Status.InfrastructureReady {
		t.Fatalf("infrastructure is not ready")
	}

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

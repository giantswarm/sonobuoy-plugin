package sonobuoy_plugin

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/conditions/pkg/conditions"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/assert"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
)

func Test_MachinePoolCR(t *testing.T) {
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

	machinePoolGetter := func(machinePoolID string) capiutil.TestedObject {
		machinePool, err := capiutil.FindMachinePool(ctx, cpCtrlClient, machinePoolID)
		if err != nil {
			t.Fatalf("error finding MachinePool %s: %s", machinePoolID, microerror.JSON(err))
		}

		return machinePool
	}

	machinePools, err := capiutil.FindNonTestingMachinePoolsForCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding MachinePools for cluster %q: %s", clusterID, microerror.JSON(err))
	}

	if len(machinePools) == 0 {
		t.Fatal("Expected one machine pool to exist, none found.")
	}

	for _, machinePool := range machinePools {
		mp := machinePool

		//
		// Check basic metadata
		//

		// Check if 'giantswarm.io/machine-pool' label is set
		assert.LabelIsSet(t, &mp, label.MachinePool)

		// Check if 'cluster.k8s.io/cluster-api-autoscaler-node-group-min-size' annotation is set
		assert.AnnotationIsSet(t, &mp, annotation.NodePoolMinSize)

		// Check if 'cluster.k8s.io/cluster-api-autoscaler-node-group-max-size' annotation is set
		assert.AnnotationIsSet(t, &mp, annotation.NodePoolMaxSize)

		// Wait for Ready condition to be True
		capiutil.WaitForCondition(t, ctx, logger, &mp, capi.ReadyCondition, capiconditions.IsTrue, machinePoolGetter)

		// Assert that MachinePool owner reference is set to the specified Cluster
		assert.ExpectedOwnerReferenceIsSet(t, &mp, cluster)

		//
		// Check Spec & Status
		//

		minReplicasString := mp.Annotations[annotation.NodePoolMinSize]
		minReplicas, err := strconv.Atoi(minReplicasString)
		if err != nil {
			t.Fatalf("error converting annotation %q to integer %v", annotation.NodePoolMinSize, err)
		}

		maxReplicasString := mp.Annotations[annotation.NodePoolMaxSize]
		maxReplicas, err := strconv.Atoi(maxReplicasString)
		if err != nil {
			t.Fatalf("error converting annotation %q to integer %v", annotation.NodePoolMaxSize, err)
		}

		// Check if number of found replicas is within expected cluster autoscaler limits
		if int(mp.Status.Replicas) < minReplicas {
			t.Fatalf("specified min %d replicas in annotation %q, found %d", minReplicas, annotation.NodePoolMinSize, mp.Status.Replicas)
		}
		if int(mp.Status.Replicas) > maxReplicas {
			t.Fatalf("specified max %d replicas in annotation %q, found %d", maxReplicas, annotation.NodePoolMaxSize, mp.Status.Replicas)
		}

		// Check if all discovered replicas are ready
		if mp.Status.Replicas != mp.Status.ReadyReplicas {
			t.Fatalf("%d replicas found, but %d are ready", mp.Status.Replicas, mp.Status.AvailableReplicas)
		}

		// Verify that InfrastructureReady condition is True
		if !conditions.IsInfrastructureReadyTrue(&mp) {
			t.Fatalf("MachinePool InfrastructureReady condition is not True")
		}

		// Verify that ReplicasReady condition is True
		if !conditions.IsReplicasReadyTrue(&mp) {
			t.Fatalf("MachinePool ReplicasReady condition is not True")
		}
	}
}

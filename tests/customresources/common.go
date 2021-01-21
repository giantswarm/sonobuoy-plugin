package customresources

import (
	"context"
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type runtimeObject interface {
	v1.Object
	runtime.Object
}

func assertLabelIsEqual(t *testing.T, referenceObject runtimeObject, otherObject runtimeObject, label string) {
	referenceLabel := referenceObject.GetLabels()[label]
	otherLabel := otherObject.GetLabels()[label]
	referenceObjectKind := referenceObject.GetObjectKind()
	otherObjectKind := otherObject.GetObjectKind()

	if otherLabel != referenceLabel {
		t.Fatalf("expected %s label %q to have value %q (to match %s CR), but got %q",
			otherObjectKind.GroupVersionKind().Kind,
			label,
			referenceLabel,
			referenceObjectKind.GroupVersionKind().Kind,
			otherLabel)
	}
}

func assertAnnotationIsEqual(t *testing.T, referenceObject runtimeObject, otherObject runtimeObject, annotation string) {
	referenceAnnotation := referenceObject.GetAnnotations()[annotation]
	otherAnnotation := otherObject.GetAnnotations()[annotation]
	referenceObjectKind := referenceObject.GetObjectKind()
	otherObjectKind := otherObject.GetObjectKind()

	if otherAnnotation != referenceAnnotation {
		t.Fatalf("expected %s annotation %q to have value %q (to match %s CR), but got %q",
			otherObjectKind.GroupVersionKind().Kind,
			annotation,
			referenceAnnotation,
			referenceObjectKind.GroupVersionKind().Kind,
			otherAnnotation)
	}
}

func getTestedCluster(ctx context.Context, t *testing.T, cpCtrlClient client.Client) *capi.Cluster {
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

type conditionCheck func(cluster capiconditions.Getter, conditionType capi.ConditionType) bool

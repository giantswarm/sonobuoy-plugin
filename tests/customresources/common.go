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

func assertLabelIsSet(t *testing.T, object runtimeObject, label string) {
	_, isAnnotationSet := object.GetLabels()[label]
	if !isAnnotationSet {
		t.Fatalf("%s '%s/%s': expected that label %q is set",
			object.GetObjectKind().GroupVersionKind().Kind,
			object.GetNamespace(),
			object.GetName(),
			label)
	}
}

func assertAnnotationIsSet(t *testing.T, object runtimeObject, annotation string) {
	_, isAnnotationSet := object.GetAnnotations()[annotation]
	if !isAnnotationSet {
		t.Fatalf("%s '%s/%s': expected that annotation %q is set",
			object.GetObjectKind().GroupVersionKind().Kind,
			object.GetNamespace(),
			object.GetName(),
			annotation)
	}
}

func assertLabelIsEqual(t *testing.T, referenceObject runtimeObject, otherObject runtimeObject, label string) {
	referenceLabel := referenceObject.GetLabels()[label]
	otherLabel := otherObject.GetLabels()[label]
	referenceObjectKind := referenceObject.GetObjectKind()
	otherObjectKind := otherObject.GetObjectKind()

	if otherLabel != referenceLabel {
		t.Fatalf("%s '%s/%s': expected label %q to have value %q (to match %s CR), but got %q",
			otherObjectKind.GroupVersionKind().Kind,
			otherObject.GetNamespace(),
			otherObject.GetName(),
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
		t.Fatalf("%s '%s/%s': expected annotation %q to have value %q (to match %s CR), but got %q",
			otherObjectKind.GroupVersionKind().Kind,
			otherObject.GetNamespace(),
			otherObject.GetName(),
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

package customresources

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
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

type conditionCheck func(cluster capiconditions.Getter, conditionType capi.ConditionType) bool

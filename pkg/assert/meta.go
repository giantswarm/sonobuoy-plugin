package assert

import (
	"testing"

	"sigs.k8s.io/cluster-api/util"
)

func LabelIsSet(t *testing.T, object TestedObject, label string) {
	_, isAnnotationSet := object.GetLabels()[label]
	if !isAnnotationSet {
		t.Fatalf("%s '%s/%s': expected that label %q is set",
			object.GetObjectKind().GroupVersionKind().Kind,
			object.GetNamespace(),
			object.GetName(),
			label)
	}
}

func AnnotationIsSet(t *testing.T, object TestedObject, annotation string) {
	_, isAnnotationSet := object.GetAnnotations()[annotation]
	if !isAnnotationSet {
		t.Fatalf("%s '%s/%s': expected that annotation %q is set",
			object.GetObjectKind().GroupVersionKind().Kind,
			object.GetNamespace(),
			object.GetName(),
			annotation)
	}
}

func LabelIsEqual(t *testing.T, referenceObject TestedObject, otherObject TestedObject, label string) {
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

func AnnotationIsEqual(t *testing.T, referenceObject TestedObject, otherObject TestedObject, annotation string) {
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

func ExpectedOwnerReferenceIsSet(t *testing.T, obj TestedObject, expectedOwner TestedObject) {
	objectName := obj.GetName()
	objectKind := obj.GetObjectKind().GroupVersionKind().Kind
	expectedOwnerKind := expectedOwner.GetObjectKind().GroupVersionKind()

	if !util.IsOwnedByObject(obj, expectedOwner) {
		t.Fatalf(
			"%s %q does not have owner reference set to %s %q",
			objectKind,
			objectName,
			expectedOwnerKind,
			expectedOwner.GetName())
	}
}

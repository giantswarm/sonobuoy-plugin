package assert

import (
	"testing"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
)

func LabelIsSet(t *testing.T, object capiutil.TestedObject, label string) {
	_, isAnnotationSet := object.GetLabels()[label]
	if !isAnnotationSet {
		t.Fatalf("%s '%s/%s': expected that label %q is set",
			object.GetObjectKind().GroupVersionKind().Kind,
			object.GetNamespace(),
			object.GetName(),
			label)
	}
}

func AnnotationIsSet(t *testing.T, object capiutil.TestedObject, annotation string) {
	_, isAnnotationSet := object.GetAnnotations()[annotation]
	if !isAnnotationSet {
		t.Fatalf("%s '%s/%s': expected that annotation %q is set",
			object.GetObjectKind().GroupVersionKind().Kind,
			object.GetNamespace(),
			object.GetName(),
			annotation)
	}
}

func LabelIsEqual(t *testing.T, referenceObject capiutil.TestedObject, otherObject capiutil.TestedObject, label string) {
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

func AnnotationIsEqual(t *testing.T, referenceObject capiutil.TestedObject, otherObject capiutil.TestedObject, annotation string) {
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

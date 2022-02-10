package capiutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/conditions"
)

type TestedObject interface {
	conditions.Setter
	metav1.Object
	runtime.Object
}

// ConditionCheck is a function interface for checking a condition of specified
// type for the specified object.
type ConditionCheck func(cluster conditions.Getter, conditionType capi.ConditionType) bool

// ObjGetterFunc is a getter function for fetching a CR with specified name.
type ObjGetterFunc func(name string) TestedObject

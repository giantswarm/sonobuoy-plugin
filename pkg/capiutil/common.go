package capiutil

import (
	"reflect"
	"testing"
	"time"

	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
)

// WaitForCondition checks if the specified object obj passes the specified
// condition check. If the check fails, it will sleep 1 minute and check again.
// When the check is passed, the function just returns.
func WaitForCondition(t *testing.T, obj TestedObject, conditionType capi.ConditionType, check ConditionCheck, objGetterFunc ObjGetterFunc) {
	objectName := obj.GetName()
	objectKind := obj.GetObjectKind().GroupVersionKind().Kind
	checkResult := check(obj, conditionType)

	for ; checkResult != true; checkResult = check(obj, conditionType) {
		currentConditionValue := capiconditions.Get(obj, conditionType)
		if currentConditionValue != nil {
			t.Logf(
				"%s %q currently has condition %q with Status=%q, Reason=%q, waiting 1 minute for desired value...",
				objectKind,
				objectName,
				conditionType,
				currentConditionValue.Status,
				currentConditionValue.Reason)
		} else {
			t.Logf(
				"%s %q does not have condition %q, waiting 1 minute for desired value...",
				objectName,
				objectKind,
				conditionType)
		}

		time.Sleep(1 * time.Minute)
		updatedObject := objGetterFunc(objectName)
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(updatedObject).Elem())
	}
}

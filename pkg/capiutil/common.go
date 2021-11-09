package capiutil

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
)

// WaitForCondition checks if the specified object obj passes the specified
// condition check. If the check fails, it will sleep 1 minute and check again.
// When the check is passed, the function just returns.
func WaitForCondition(t *testing.T, ctx context.Context, logger micrologger.Logger, obj TestedObject, conditionType capi.ConditionType, check ConditionCheck, objGetterFunc ObjGetterFunc) {
	objectName := obj.GetName()
	objectKind := obj.GetObjectKind().GroupVersionKind().Kind
	retryInterval := backoff.LongMaxInterval

	o := func() error {
		updatedObject := objGetterFunc(objectName)

		// Underlying type for TestedObject is a pointer to some struct, so this
		// is like saying:
		//
		//    *obj = *updatedObject
		//
		// The object obj that is passed into the function will be modified.
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(updatedObject).Elem())

		conditionCheckPassed := check(obj, conditionType)
		if conditionCheckPassed {
			return nil
		}

		currentConditionValue := capiconditions.Get(obj, conditionType)
		if currentConditionValue != nil {
			return microerror.Maskf(
				unexpectedConditionValueError,
				"%s %q currently has condition %q with Status=%q, Reason=%q, waiting %s before checking again...",
				objectKind,
				objectName,
				conditionType,
				currentConditionValue.Status,
				currentConditionValue.Reason,
				retryInterval)
		} else {
			return microerror.Maskf(
				unexpectedConditionValueError,
				"%s %q does not have condition %q, waiting %s before checking again...",
				objectName,
				objectKind,
				conditionType,
				retryInterval)
		}
	}

	b := backoff.NewExponential(20*time.Minute, retryInterval)
	n := backoff.NewNotifier(logger, ctx)
	err := backoff.RetryNotify(o, b, n)
	if err != nil {
		t.Fatalf(
			"error while waiting for condition %q on %s %q",
			conditionType,
			objectKind,
			objectName)
	}
}

func GetCondition(obj TestedObject, conditionType capi.ConditionType) corev1.ConditionStatus {
	cond := capiconditions.Get(obj, conditionType)
	if cond == nil {
		return corev1.ConditionUnknown
	} else {
		return cond.Status
	}
}

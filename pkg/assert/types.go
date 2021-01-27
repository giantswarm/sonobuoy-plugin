package assert

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type TestedObject interface {
	metav1.Object
	runtime.Object
}

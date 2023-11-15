package sonobuoy_plugin

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/api/kyverno/v2beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

const (
	pvcNamespace = "default"
)

// Test_PVC tests PVCs with default storage class are being provisioned.
func Test_PVC(t *testing.T) {
	t.Parallel()

	var err error

	ctx := context.Background()

	tcCtrlClient, err := ctrlclient.CreateTCCtrlClient()
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	regularLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := NewTestLogger(regularLogger, t)

	classes, err := getStorageClasses(ctx, tcCtrlClient)
	if err != nil {
		t.Fatal(err)
	}

	for _, class := range classes {
		pvc, err := createPVC(ctx, tcCtrlClient, class)
		if err != nil {
			t.Fatal(err)
		}

		polex, err := createPolex(ctx, tcCtrlClient, pvc)
		if err != nil {
			t.Fatal(err)
		}

		pod, err := createPod(ctx, tcCtrlClient, pvc)
		if err != nil {
			t.Fatal(err)
		}

		cleanup := func() {
			_ = tcCtrlClient.Delete(ctx, pvc)
			_ = tcCtrlClient.Delete(ctx, polex)
			_ = tcCtrlClient.Delete(ctx, pod)
		}

		// Wait for the PVC to be bound.
		o := func() error {
			current := &corev1.PersistentVolumeClaim{}
			err := tcCtrlClient.Get(ctx, client.ObjectKey{Name: pvc.Name, Namespace: pvc.Namespace}, current)
			if err != nil {
				t.Fatal(err)
			}

			if current.Status.Phase != corev1.ClaimBound {
				return microerror.Maskf(pvcUnboundError, "PVC for storage class %q is in phase %s", class, current.Status.Phase)
			}

			return nil
		}

		b := backoff.NewConstant(5*time.Minute, 10*time.Second)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			cleanup()
			t.Fatalf("timeout waiting for PVC to be bound: %v", err)
		}

		cleanup()
	}
}

func getStorageClasses(ctx context.Context, ctrlClient client.Client) ([]string, error) {

	var storageclasses v1.StorageClassList
	err := ctrlClient.List(ctx, &storageclasses)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	// Empty string means default storage class
	ret := []string{""}
	for _, sc := range storageclasses.Items {
		ret = append(ret, sc.Name)
	}

	return ret, nil
}

func createPVC(ctx context.Context, ctrlClient client.Client, storageClass string) (*corev1.PersistentVolumeClaim, error) {
	pvm := corev1.PersistentVolumeFilesystem

	name := "mypvc"
	if storageClass != "" {
		name = fmt.Sprintf("%s-%s", name, storageClass)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pvcNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadWriteOnce",
			},
			VolumeMode: &pvm,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("8Gi"),
				},
			},
		},
	}
	if storageClass != "" {
		pvc.Spec.StorageClassName = &storageClass

	}
	err := ctrlClient.Create(ctx, pvc)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func createPolex(ctx context.Context, ctrlClient client.Client, pvc *corev1.PersistentVolumeClaim) (*v2alpha1.PolicyException, error) {
	polex := &v2alpha1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("pvc-test-%s", pvc.Name),
			Namespace: "giantswarm",
		},
		Spec: v2alpha1.PolicyExceptionSpec{
			Match: v2beta1.MatchResources{
				Any: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds:      []string{"Pod"},
							Names:      []string{pvc.Name},
							Namespaces: []string{pvcNamespace},
						},
					},
				},
			},
			Exceptions: []v2alpha1.Exception{
				{
					PolicyName: "disallow-capabilities-strict",
					RuleNames:  []string{"require-drop-all"},
				},
				{
					PolicyName: "disallow-privilege-escalation",
					RuleNames:  []string{"privilege-escalation"},
				},
				{
					PolicyName: "require-run-as-nonroot",
					RuleNames:  []string{"run-as-non-root"},
				},
				{
					PolicyName: "restrict-seccomp-strict",
					RuleNames:  []string{"check-seccomp-strict"},
				},
			},
		},
	}

	err := ctrlClient.Create(ctx, polex)
	if err != nil {
		return nil, err
	}

	return polex, nil
}

func createPod(ctx context.Context, ctrlClient client.Client, pvc *corev1.PersistentVolumeClaim) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvc.Name,
			Namespace: pvcNamespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "mypod",
					Image: "quay.io/giantswarm/helloworld:latest",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "mypv",
							MountPath: "/mnt",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "mypv",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				},
			},
		},
	}
	err := ctrlClient.Create(ctx, pod)
	if err != nil {
		return nil, err
	}

	return pod, nil
}

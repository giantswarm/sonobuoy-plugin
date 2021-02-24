package sonobuoy_plugin

import (
	"context"
	"testing"
	"time"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

const (
	pvcName      = "mypvc"
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

	pvc, err := createPVC(ctx, tcCtrlClient)
	if err != nil {
		t.Fatal(err)
	}

	pod, err := createPod(ctx, tcCtrlClient)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = tcCtrlClient.Delete(ctx, pvc)
		_ = tcCtrlClient.Delete(ctx, pod)
	})

	// Wait for the PVC to be bound.
	o := func() error {
		current := &corev1.PersistentVolumeClaim{}
		err := tcCtrlClient.Get(ctx, client.ObjectKey{Name: pvc.Name, Namespace: pvc.Namespace}, current)
		if err != nil {
			t.Fatal(err)
		}

		if current.Status.Phase != corev1.ClaimBound {
			return microerror.Maskf(pvcUnboundError, "PVC is in phase %s", current.Status.Phase)
		}

		return nil
	}

	b := backoff.NewConstant(5*time.Minute, 10*time.Second)
	n := backoff.NewNotifier(logger, ctx)
	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		t.Fatalf("timeout waiting for PVC to be bound: %v", err)
	}
}

func createPVC(ctx context.Context, ctrlClient client.Client) (*corev1.PersistentVolumeClaim, error) {
	pvm := corev1.PersistentVolumeFilesystem

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
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
	err := ctrlClient.Create(ctx, pvc)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func createPod(ctx context.Context, ctrlClient client.Client) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
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
							ClaimName: pvcName,
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

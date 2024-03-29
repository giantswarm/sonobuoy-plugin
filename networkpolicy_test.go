package sonobuoy_plugin

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/api/kyverno/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

const (
	successfulPodName = "np-success"
	failurePodName    = "np-failure"
	npTestNamespace   = "default"
)

// Test_Autoscaler checks the Cluster Autoscaler works by creating a deployment with PodAntiAffinity and scaling it up and down.
func Test_NetworkPolicy(t *testing.T) {
	t.Parallel()

	var err error

	ctx := context.Background()

	tcCtrlClient, err := ctrlclient.CreateTCCtrlClient()
	if err != nil {
		t.Fatalf("error creating TC k8s client: %v", err)
	}

	regularLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := NewTestLogger(regularLogger, t)

	logger.Debugf(ctx, "Testing network policies")

	networkPolicies, polexes, pods, err := createPodsAndNPs(ctx, tcCtrlClient)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		for _, netpol := range networkPolicies {
			_ = tcCtrlClient.Delete(ctx, netpol)
		}
		for _, polex := range polexes {
			_ = tcCtrlClient.Delete(ctx, polex)
		}
		for _, pod := range pods {
			_ = tcCtrlClient.Delete(ctx, pod)
		}
	})

	// Successful pod.
	{
		o := func() error {
			pod := &corev1.Pod{}

			err = tcCtrlClient.Get(ctx, client.ObjectKey{Name: successfulPodName, Namespace: npTestNamespace}, pod)
			if err != nil {
				t.Fatal(err)
			}

			if len(pod.Status.ContainerStatuses) == 0 {
				return fmt.Errorf("expected pod %s to be have 'ContainerStatuses' but it still has none", successfulPodName)
			}

			cs := pod.Status.ContainerStatuses[0]

			if cs.State.Terminated != nil {
				if cs.State.Terminated.ExitCode == 0 {
					// Completed successfully
					return nil
				}

				return fmt.Errorf("expected exit code to be 0, got %d", cs.State.Terminated.ExitCode)
			} else {
				return fmt.Errorf("expected container 0 in pod %s to be terminated but was not", successfulPodName)
			}
		}

		b := backoff.NewConstant(backoff.ShortMaxWait, 10*time.Second)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("timeout waiting for pod %s to terminate successfully: %v", successfulPodName, err)
		}
	}

	// Failure pod.
	{
		o := func() error {
			pod := &corev1.Pod{}

			err = tcCtrlClient.Get(ctx, client.ObjectKey{Name: failurePodName, Namespace: npTestNamespace}, pod)
			if err != nil {
				t.Fatal(err)
			}

			cs := pod.Status.ContainerStatuses[0]

			if cs.State.Terminated != nil {
				if cs.State.Terminated.ExitCode != 0 {
					// Completed with error, expected behaviour
					return nil
				}

				return fmt.Errorf("expected exit code for pod %s not to be 0, got %d", failurePodName, cs.State.Terminated.ExitCode)
			} else {
				return fmt.Errorf("expected container 0 in pod %s to be terminated but was not", failurePodName)
			}
		}

		b := backoff.NewConstant(backoff.ShortMaxWait, 10*time.Second)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("timeout waiting for pod %s to crash: %v", successfulPodName, err)
		}
	}
}

func createPodsAndNPs(ctx context.Context, ctrlClient client.Client) ([]*networkingv1.NetworkPolicy, []*v2alpha1.PolicyException, []*corev1.Pod, error) {
	var networkPolicies []*networkingv1.NetworkPolicy
	var pods []*corev1.Pod
	var polexes []*v2alpha1.PolicyException

	labels := map[string]string{
		"test": "network-policy-test",
	}

	udp := corev1.ProtocolUDP
	tcp := corev1.ProtocolTCP

	getPortPtr := func(port int) *intstr.IntOrString {
		r := intstr.FromInt(port)
		return &r
	}

	networkPolicies = append(networkPolicies, &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "network-policy-test",
			Namespace: npTestNamespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				// Plain coredns and node-local dns cache
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &udp,
							Port:     getPortPtr(1053),
						}, {
							Protocol: &udp,
							Port:     getPortPtr(53),
						},
						{
							Protocol: &tcp,
							Port:     getPortPtr(1053),
						}, {
							Protocol: &tcp,
							Port:     getPortPtr(53),
						},
					},
				},
				// For this test.
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: "0.0.0.0/0",
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     getPortPtr(80),
						},
					},
				},
			},
		},
	})

	// PSS
	{
		polex := v2alpha1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "networkpolicy-test",
				Namespace: "giantswarm",
			},
			Spec: v2alpha1.PolicyExceptionSpec{
				Match: v2beta1.MatchResources{
					Any: v1.ResourceFilters{
						{
							ResourceDescription: v1.ResourceDescription{
								Kinds:      []string{"Pod"},
								Names:      []string{successfulPodName, failurePodName},
								Namespaces: []string{npTestNamespace},
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

		polexes = append(polexes, &polex)
	}

	// Successful pod and NetworkPolicy.
	{
		pods = append(pods, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      successfulPodName,
				Namespace: npTestNamespace,
				Labels:    labels,
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:  "curl",
						Image: "quay.io/giantswarm/alpine-curl:latest",
						// Succeedes because it uses http (port 80 allowed)
						Command: []string{
							"curl",
							"http://www.amazonaws.cn",
							"-I",
							"-m",
							"10",
						},
						ImagePullPolicy: "Always",
					},
				},
			},
		})
	}

	// Failure pod and NetworkPolicy.
	{
		pods = append(pods, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      failurePodName,
				Namespace: npTestNamespace,
				Labels:    labels,
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:  "curl",
						Image: "quay.io/giantswarm/alpine-curl:latest",
						// Fails because it uses https (port 443 not allowed)
						Command: []string{
							"curl",
							"https://www.amazonaws.cn",
							"-I",
							"-m",
							"10",
						},
						ImagePullPolicy: "Always",
					},
				},
			},
		})
	}

	for _, obj := range networkPolicies {
		// Delete the object in case it's there to allow for running test more than once.
		_ = ctrlClient.Delete(ctx, obj)

		err := ctrlClient.Create(ctx, obj)
		if err != nil {
			return nil, nil, nil, microerror.Mask(err)
		}
	}

	for _, obj := range polexes {
		// Delete the object in case it's there to allow for running test more than once.
		_ = ctrlClient.Delete(ctx, obj)

		err := ctrlClient.Create(ctx, obj)
		if err != nil {
			return nil, nil, nil, microerror.Mask(err)
		}
	}

	for _, obj := range pods {
		// Delete the object in case it's there to allow for running test more than once.
		_ = ctrlClient.Delete(ctx, obj)

		err := ctrlClient.Create(ctx, obj)
		if err != nil {
			return nil, nil, nil, microerror.Mask(err)
		}
	}

	return networkPolicies, polexes, pods, nil
}

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/ghodss/yaml"
	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/apis/infrastructure/v1alpha3"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/randomid"
)

const (
	awsRegion = "eu-central-1"
)

type AWSProviderSupport struct {
	logger micrologger.Logger

	ec2Client *ec2.EC2
}

func NewAWSProviderSupport(ctx context.Context, logger micrologger.Logger, client ctrl.Client, cluster *capi.Cluster) (Support, error) {
	var awsCluster v1alpha3.AWSCluster
	{
		err := client.Get(ctx, ctrl.ObjectKey{Name: cluster.Spec.InfrastructureRef.Name, Namespace: cluster.Spec.InfrastructureRef.Namespace}, &awsCluster)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	ec2Client, err := getEc2Client(ctx, client, awsCluster)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	p := &AWSProviderSupport{
		logger:    logger,
		ec2Client: ec2Client,
	}

	return p, nil
}

func (p *AWSProviderSupport) CreateNodePoolAndWaitReady(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azs []string) (*ctrl.ObjectKey, error) {
	awsMP, err := p.createAwsMachineDeployment(ctx, client, cluster, azs)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	mp, err := p.createMachineDeployment(ctx, client, cluster, awsMP, azs)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	p.logger.Debugf(ctx, "Created machine deployment %s", mp.Name)

	// Wait for Node Pool to come up.
	{
		o := func() error {
			err := client.Get(ctx, ctrl.ObjectKey{Name: mp.Name, Namespace: mp.Namespace}, mp)
			if err != nil {
				// Wrap masked error with backoff.Permanent() to stop retries on unrecoverable error.
				return backoff.Permanent(microerror.Mask(err))
			}

			// Return error for retry until node pool nodes are Ready.
			if mp.Status.Replicas == mp.Status.ReadyReplicas && mp.Status.ReadyReplicas > 0 {
				return nil
			}

			return errors.New("node pool is not ready yet")
		}
		b := backoff.NewConstant(backoff.LongMaxWait, backoff.LongMaxInterval)
		n := backoff.NewNotifier(p.logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			return nil, microerror.Mask(fmt.Errorf("failed to get MachinePool %q for Cluster %q: %s", mp.Name, cluster.Name, microerror.JSON(err)))
		}
	}

	return &ctrl.ObjectKey{Name: mp.Name, Namespace: mp.Namespace}, nil
}

func (p *AWSProviderSupport) DeleteNodePool(ctx context.Context, client ctrl.Client, objKey ctrl.ObjectKey) error {
	mp := &capi.MachineDeployment{}

	err := client.Get(ctx, objKey, mp)
	if err != nil {
		return microerror.Mask(err)
	}

	return client.Delete(ctx, mp)
}

func (p *AWSProviderSupport) GetNodeSelectorLabel() string {
	return "giantswarm.io/machine-deployment"
}

func (p *AWSProviderSupport) GetTestingMachinePoolForCluster(ctx context.Context, client ctrl.Client, clusterID string) (string, error) {
	var machinePoolName string
	{
		var machinePools []capi.MachineDeployment
		{
			var machinePoolList capi.MachineDeploymentList
			err := client.List(ctx, &machinePoolList, ctrl.MatchingLabels{capi.ClusterLabelName: clusterID})
			if err != nil {
				return "", microerror.Mask(err)
			}

			for _, machinePool := range machinePoolList.Items {
				_, isE2E := machinePool.Labels[capiutil.E2ENodepool]
				if isE2E {
					continue
				}

				machinePools = append(machinePools, machinePool)
			}
		}

		if len(machinePools) == 0 {
			return "", fmt.Errorf("expected one machine pool to exist, none found")
		}

		machinePoolName = machinePools[0].Name
	}

	return machinePoolName, nil
}

func (p *AWSProviderSupport) GetProviderAZs() []string {
	return []string{
		fmt.Sprintf("%sa", awsRegion),
		fmt.Sprintf("%sb", awsRegion),
		fmt.Sprintf("%sc", awsRegion),
	}
}

func (p *AWSProviderSupport) GetNodePoolAZsInCR(ctx context.Context, client ctrl.Client, objKey ctrl.ObjectKey) ([]string, error) {
	mp := &v1alpha3.AWSMachineDeployment{}

	err := client.Get(ctx, objKey, mp)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return mp.Spec.Provider.AvailabilityZones, nil
}

func (p *AWSProviderSupport) GetNodePoolAZsInProvider(ctx context.Context, clusterID, nodepoolID string) ([]string, error) {
	var zones []string

	err := p.ec2Client.DescribeInstancesPages(
		&ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name: aws.String(fmt.Sprintf("tag:%s", label.MachineDeployment)),
					Values: []*string{
						aws.String(nodepoolID),
					},
				},
			},
		},
		func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
			for _, res := range page.Reservations {
				if res != nil {
					for _, instance := range res.Instances {
						if instance != nil && instance.Placement != nil && instance.Placement.AvailabilityZone != nil {
							zones = append(zones, *instance.Placement.AvailabilityZone)
						}
					}
				}
			}
			return true
		},
	)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return zones, nil
}

func (p *AWSProviderSupport) createMachineDeployment(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, awsMachineDeployment *v1alpha3.AWSMachineDeployment, azs []string) (*capi.MachineDeployment, error) {
	var infrastructureCRRef *corev1.ObjectReference
	{
		s := runtime.NewScheme()
		err := v1alpha3.AddToScheme(s)
		if err != nil {
			panic(fmt.Sprintf("capav1alpha3.AddToScheme: %+v", err))
		}

		infrastructureCRRef, err = reference.GetReference(s, awsMachineDeployment)
		if err != nil {
			panic(fmt.Sprintf("cannot create reference to infrastructure CR: %q", err))
		}
	}

	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      awsMachineDeployment.Name,
			Namespace: awsMachineDeployment.Namespace,
			Labels: map[string]string{
				label.AWSOperatorVersion: awsMachineDeployment.Labels[label.AWSOperatorVersion],
				label.Cluster:            cluster.Labels[label.Cluster],
				capi.ClusterLabelName:    cluster.Labels[capi.ClusterLabelName],
				label.MachineDeployment:  awsMachineDeployment.Labels[label.MachineDeployment],
				label.Organization:       cluster.Labels[label.Organization],
				label.ReleaseVersion:     cluster.Labels[label.ReleaseVersion],
				capiutil.E2ENodepool:     "true",
			},
			Annotations: map[string]string{
				annotation.MachinePoolName: "availability zone verification e2e test",
				annotation.NodePoolMinSize: fmt.Sprintf("%d", len(azs)),
				annotation.NodePoolMaxSize: fmt.Sprintf("%d", len(azs)),
			},
		},
		Spec: capi.MachineDeploymentSpec{
			ClusterName: cluster.Name,
			Replicas:    to.Int32Ptr(0),
			Template: capi.MachineTemplateSpec{
				Spec: capi.MachineSpec{
					ClusterName:       cluster.Name,
					InfrastructureRef: *infrastructureCRRef,
				},
			},
		},
	}

	err := client.Create(ctx, machineDeployment)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return machineDeployment, nil
}

func (p *AWSProviderSupport) createAwsMachineDeployment(ctx context.Context, client ctrl.Client, cluster *capi.Cluster, azs []string) (*v1alpha3.AWSMachineDeployment, error) {
	nodepoolName := randomid.New()

	awscluster := v1alpha3.AWSCluster{}
	err := client.Get(ctx, ctrl.ObjectKey{Name: cluster.Spec.InfrastructureRef.Name, Namespace: cluster.Spec.InfrastructureRef.Namespace}, &awscluster)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	awsMachineDeployment := &v1alpha3.AWSMachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodepoolName,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				label.AWSOperatorVersion: awscluster.Labels[label.AWSOperatorVersion],
				label.Cluster:            cluster.Labels[label.Cluster],
				capi.ClusterLabelName:    cluster.Name,
				label.MachineDeployment:  nodepoolName,
				label.Organization:       cluster.Labels[label.Organization],
				label.ReleaseVersion:     cluster.Labels[label.ReleaseVersion],
				capiutil.E2ENodepool:     "true",
			},
		},
		Spec: v1alpha3.AWSMachineDeploymentSpec{
			NodePool: v1alpha3.AWSMachineDeploymentSpecNodePool{
				Description: nodepoolName,
				Machine: v1alpha3.AWSMachineDeploymentSpecNodePoolMachine{
					DockerVolumeSizeGB:  100,
					KubeletVolumeSizeGB: 100,
				},
				Scaling: v1alpha3.AWSMachineDeploymentSpecNodePoolScaling{
					Max: len(azs),
					Min: len(azs),
				},
			},
			Provider: v1alpha3.AWSMachineDeploymentSpecProvider{
				AvailabilityZones: azs,
				InstanceDistribution: v1alpha3.AWSMachineDeploymentSpecInstanceDistribution{
					OnDemandBaseCapacity:                0,
					OnDemandPercentageAboveBaseCapacity: to.IntPtr(100),
				},
				Worker: v1alpha3.AWSMachineDeploymentSpecProviderWorker{
					InstanceType:          "m5.xlarge",
					UseAlikeInstanceTypes: false,
				},
			},
		},
	}
	err = client.Create(ctx, awsMachineDeployment)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return awsMachineDeployment, nil
}

func getEc2Client(ctx context.Context, client ctrl.Client, awsCluster v1alpha3.AWSCluster) (*ec2.EC2, error) {
	var err error

	arn, err := getARN(ctx, client, awsCluster)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	accessKeyID, accessKeySecret, err := getAWSCredentialsFromAwsOperatorSecret(ctx, client, awsCluster)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	region := awsRegion
	sessionToken := ""

	var s *session.Session
	{
		c := &aws.Config{
			Credentials: credentials.NewStaticCredentials(accessKeyID, accessKeySecret, sessionToken),
			Region:      aws.String(region),
		}

		s, err = session.NewSession(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	credentialsConfig := &aws.Config{
		Credentials: stscreds.NewCredentials(s, arn),
	}

	return ec2.New(s, credentialsConfig), nil
}

func getARN(ctx context.Context, client ctrl.Client, awsCluster v1alpha3.AWSCluster) (string, error) {
	var err error

	credential := &corev1.Secret{}
	{
		credentialName := awsCluster.Spec.Provider.CredentialSecret.Name
		if credentialName == "" {
			return "", errors.New("AWSCluster.Spec.Provider.CredentialSecret.Name was empty")
		}

		credentialNamespace := awsCluster.Spec.Provider.CredentialSecret.Namespace
		if credentialName == "" {
			return "", errors.New("AWSCluster.Spec.Provider.CredentialSecret.Namespace was empty")
		}

		err = client.Get(ctx, ctrl.ObjectKey{Namespace: credentialNamespace, Name: credentialName}, credential)
		if err != nil {
			return "", microerror.Mask(err)
		}
	}

	arn, ok := credential.Data["aws.awsoperator.arn"]
	if !ok {
		return "", errors.New("Unable to find ARN")
	}

	return string(arn), nil
}

func getAWSCredentialsFromAwsOperatorSecret(ctx context.Context, client ctrl.Client, awscluster v1alpha3.AWSCluster) (string, string, error) {
	secrets := &corev1.SecretList{}
	err := client.List(ctx, secrets, ctrl.MatchingLabels{
		label.App:                  "aws-operator",
		label.AppKubernetesVersion: awscluster.Labels[label.AWSOperatorVersion],
	})
	if err != nil {
		return "", "", microerror.Mask(err)
	}

	type conf struct {
		Service struct {
			AWS struct {
				HostAccessKey struct {
					ID     string `yaml:"id"`
					Secret string `yaml:"secret"`
				} `yaml:"hostAccessKey"`
			} `yaml:"aws"`
		} `yaml:"service"`
	}

	for _, secret := range secrets.Items {
		wantedKey := "aws-secret.yaml"
		if raw := secret.Data[wantedKey]; raw != nil {
			// Something like this:
			// service:
			//   aws:
			//     hostAccessKey:
			//       id: ...
			//       secret: ...

			val := conf{}
			err = yaml.Unmarshal(raw, &val)
			if err != nil {
				return "", "", microerror.Mask(fmt.Errorf("unable to decode aws-operator secret: %s", err))
			}

			return val.Service.AWS.HostAccessKey.ID, val.Service.AWS.HostAccessKey.Secret, nil
		}
	}

	return "", "", microerror.Mask(fmt.Errorf("can't find valid aws-operator secret with %q=%q and %q=%q", label.App, "aws-operator", label.AppKubernetesVersion, awscluster.Labels[label.AWSOperatorVersion]))
}

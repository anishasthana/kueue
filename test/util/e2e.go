package util

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	jobset "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	kueuealpha "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	visibility "sigs.k8s.io/kueue/apis/visibility/v1alpha1"
	kueueclientset "sigs.k8s.io/kueue/client-go/clientset/versioned"
	visibilityv1alpha1 "sigs.k8s.io/kueue/client-go/clientset/versioned/typed/visibility/v1alpha1"
)

const (
	// The environment variable for namespace where Kueue is installed
	namespaceEnvVar = "NAMESPACE"

	// The namespace where kueue is installed in opendatahub
	odhNamespace = "opendatahub"

	// The namespace where kueue is installed in rhoai
	rhoaiNamespace = "redhat-ods-applications"

	// The default namespace where kueue is installed
	kueueNamespace = "kueue-system"

	undefinedNamespace = "undefined"
)

func CreateClientUsingCluster(kContext string) (client.WithWatch, *rest.Config) {
	cfg, err := config.GetConfigWithContext(kContext)
	if err != nil {
		fmt.Printf("unable to get kubeconfig for context %q: %s", kContext, err)
		os.Exit(1)
	}
	gomega.ExpectWithOffset(1, cfg).NotTo(gomega.BeNil())

	err = kueue.AddToScheme(scheme.Scheme)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

	err = kueuealpha.AddToScheme(scheme.Scheme)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

	err = visibility.AddToScheme(scheme.Scheme)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

	err = jobset.AddToScheme(scheme.Scheme)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

	client, err := client.NewWithWatch(cfg, client.Options{Scheme: scheme.Scheme})
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	return client, cfg
}

func CreateVisibilityClient(user string) visibilityv1alpha1.VisibilityV1alpha1Interface {
	cfg, err := config.GetConfigWithContext("")
	if err != nil {
		fmt.Printf("unable to get kubeconfig: %s", err)
		os.Exit(1)
	}
	gomega.ExpectWithOffset(1, cfg).NotTo(gomega.BeNil())

	if user != "" {
		cfg.Impersonate.UserName = user
	}

	kueueClient, err := kueueclientset.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("unable to create kueue clientset: %s", err)
		os.Exit(1)
	}
	visibilityClient := kueueClient.VisibilityV1alpha1()
	return visibilityClient
}

func waitForOperatorAvailability(ctx context.Context, k8sClient client.Client, key types.NamespacedName) {
	deployment := &appsv1.Deployment{}
	pods := &corev1.PodList{}
	gomega.EventuallyWithOffset(2, func(g gomega.Gomega) error {
		g.Expect(k8sClient.Get(ctx, key, deployment)).To(gomega.Succeed())
		g.Expect(k8sClient.List(ctx, pods, client.InNamespace(GetNamespace()), client.MatchingLabels(deployment.Spec.Selector.MatchLabels))).To(gomega.Succeed())
		g.Expect(deployment.Status.Conditions).To(gomega.ContainElement(gomega.BeComparableTo(
			appsv1.DeploymentCondition{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
			cmpopts.IgnoreFields(appsv1.DeploymentCondition{}, "Reason", "Message", "LastUpdateTime", "LastTransitionTime")),
		))
		return nil
	}, StartUpTimeout, Interval).Should(gomega.Succeed())
}

func WaitForKueueAvailability(ctx context.Context, k8sClient client.Client) {
	kcmKey := types.NamespacedName{Namespace: GetNamespace(), Name: "kueue-controller-manager"}
	waitForOperatorAvailability(ctx, k8sClient, kcmKey)
}

func WaitForJobSetAvailability(ctx context.Context, k8sClient client.Client) {
	kcmKey := types.NamespacedName{Namespace: "jobset-system", Name: "jobset-controller-manager"}
	waitForOperatorAvailability(ctx, k8sClient, kcmKey)
}

func GetNamespace() string {
	namespace, ok := os.LookupEnv(namespaceEnvVar)
	if !ok {
		fmt.Printf("Expected environment variable %s is unset, please use this environment variable to specify in which namespace Kueue is installed", namespaceEnvVar)
		os.Exit(1)
	}
	switch namespace {
	case "opendatahub":
		return odhNamespace
	case "redhat-ods-applications":
		return rhoaiNamespace
	case "kueue-system":
		return kueueNamespace
	default:
		fmt.Printf("Expected environment variable %s contains an incorrect value", namespaceEnvVar)
		os.Exit(1)
		return undefinedNamespace
	}
}

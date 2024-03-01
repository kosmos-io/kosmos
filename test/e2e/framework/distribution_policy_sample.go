// nolint:dupl
package framework

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

const (
	Distribution        = "kosmos-namespace-distribution-policy"
	ClusterDistribution = "kosmos-cluster-distribution-policy"
	NameScopeNs         = "test-name-scope"
	NamespaceScopeNs    = "test-namespace-scope"

	PodName                   = "nginx"
	PodNamePrefix             = "nginx-prefix"
	PodNameLabelScopeNs       = "nginx-test-name-label-scope"
	PodNameClusterLevel       = "cluster-nginx"
	PodNamePrefixClusterLevel = "cluster-nginx-prefix"

	SchedulerName = "default-scheduler"
)

var (
	ClusterLabel = map[string]string{
		"example-distribution-policy": "cluster-nginx",
	}

	NamespaceLabel = map[string]string{
		"example-distribution-policy": "namespace-nginx",
	}

	PodLabel = map[string]string{
		"example-distribution-policy": "nginx",
	}

	NodeSelector = map[string]string{
		"advNode": "true",
	}
)

func NewDistributionPolicy() *kosmosv1alpha1.DistributionPolicy {
	policies := make([]kosmosv1alpha1.PolicyTerm, 0)
	policies = append(policies,
		kosmosv1alpha1.PolicyTerm{
			Name:     "host",
			NodeType: "host",
		},
		kosmosv1alpha1.PolicyTerm{
			Name:     "leaf",
			NodeType: "leaf",
		},
		kosmosv1alpha1.PolicyTerm{
			Name:     "mix",
			NodeType: "mix",
		},
		kosmosv1alpha1.PolicyTerm{
			Name:     "adv",
			NodeType: "adv",
			AdvancedTerm: kosmosv1alpha1.AdvancedTerm{
				NodeSelector: NodeSelector,
			},
		})

	resources := make([]kosmosv1alpha1.ResourceSelector, 0)
	resources = append(resources,
		kosmosv1alpha1.ResourceSelector{
			PolicyName: "leaf",
			Name:       PodName,
		},
		kosmosv1alpha1.ResourceSelector{
			PolicyName: "host",
			NamePrefix: PodNamePrefix,
		},
		kosmosv1alpha1.ResourceSelector{
			PolicyName: "mix",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: PodLabel,
			},
		})

	return &kosmosv1alpha1.DistributionPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "kosmos.io/v1alpha1",
			APIVersion: "DistributionPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: Distribution,
		},
		DistributionSpec: kosmosv1alpha1.DistributionSpec{
			ResourceSelectors: resources,
			PolicyTerms:       policies,
		},
	}
}

func NewClusterDistributionPolicy() *kosmosv1alpha1.ClusterDistributionPolicy {
	policies := make([]kosmosv1alpha1.PolicyTerm, 0)
	policies = append(policies,
		kosmosv1alpha1.PolicyTerm{
			Name:     "host",
			NodeType: "host",
		},
		kosmosv1alpha1.PolicyTerm{
			Name:     "leaf",
			NodeType: "leaf",
		},
		kosmosv1alpha1.PolicyTerm{
			Name:     "mix",
			NodeType: "mix",
		},
		kosmosv1alpha1.PolicyTerm{
			Name:     "adv",
			NodeType: "adv",
			AdvancedTerm: kosmosv1alpha1.AdvancedTerm{
				NodeSelector: NodeSelector,
			},
		})

	resources := make([]kosmosv1alpha1.ResourceSelector, 0)
	resources = append(resources,
		kosmosv1alpha1.ResourceSelector{
			PolicyName: "mix",
			Name:       PodNameClusterLevel,
		},
		kosmosv1alpha1.ResourceSelector{
			PolicyName: "host",
			NamePrefix: PodNamePrefixClusterLevel,
		},
		kosmosv1alpha1.ResourceSelector{
			PolicyName: "leaf",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: ClusterLabel,
			},
		})

	return &kosmosv1alpha1.ClusterDistributionPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "kosmos.io/v1alpha1",
			APIVersion: "ClusterDistributionPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterDistribution,
		},
		DistributionSpec: kosmosv1alpha1.DistributionSpec{
			ResourceSelectors: resources,
			PolicyTerms:       policies,
		},
	}
}

func CreateDistributionPolicy(client versioned.Interface, ns string, dp *kosmosv1alpha1.DistributionPolicy) {
	ginkgo.By("Creating DistributionPolicy", func() {
		_, err := client.KosmosV1alpha1().DistributionPolicies(ns).Create(context.TODO(), dp, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			klog.Errorf("create DistributionPolicy occur error ：", err)
			gomega.Expect(err).Should(gomega.HaveOccurred())
		}
	})
}

func CreateClusterDistributionPolicy(client versioned.Interface, cdp *kosmosv1alpha1.ClusterDistributionPolicy) {
	ginkgo.By("Creating ClusterDistributionPolicy", func() {
		_, err := client.KosmosV1alpha1().ClusterDistributionPolicies().Create(context.TODO(), cdp, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			klog.Errorf("create ClusterDistributionPolicy occur error ：", err)
			gomega.Expect(err).Should(gomega.HaveOccurred())
		}
	})
}

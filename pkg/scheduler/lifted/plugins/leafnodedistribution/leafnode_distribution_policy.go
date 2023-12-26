package leafnodedistribution

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/kosmos.io/kosmos/pkg/apis/config"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

var _ framework.FilterPlugin = &LeafNodeDistribution{}

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "LeafNodeDistribution"

	DistributionPolicy = "kosmos-node-distribution-policy"

	KosmosNodeLabel = "kosmos.io/node"
)

const (
	NoneScope int = iota
	NamespaceLabelSelectorScope
	NamespacePrefixScope
	NamespaceScope
	LabelSelectorScope
	NamePrefixScope
	NameScope
	LabelSelectorInNsScope
	NamePrefixInNsScope
	NameInNsScope
)

// LeafNodeDistribution is a leaf node scheduler plugin.
type LeafNodeDistribution struct {
	handle       framework.Handle
	kosmosClient *versioned.Clientset
}

// Name returns name of the plugin.
func (knd *LeafNodeDistribution) Name() string {
	return Name
}

func getKosmosClient(ctx context.Context, kubeConfigPath *string) (client *versioned.Clientset, err error) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", *kubeConfigPath)
	if err != nil {
		klog.ErrorS(err, "Cannot create kubeconfig", "kubeConfigPath", *kubeConfigPath)
		return nil, err
	}

	client, err = versioned.NewForConfig(kubeConfig)
	if err != nil {
		klog.ErrorS(err, "cannot create client for LeafNodeDistribution", "kubeConfig", kubeConfig)
		return client, err
	}
	return client, nil
}

// New initializes a new plugin and returns it.
func New(plArgs runtime.Object, fh framework.Handle) (framework.Plugin, error) {
	klog.V(5).InfoS("Creating new LeafNodeDistribution plugin")
	dcfg, ok := plArgs.(*config.LeafNodeDistributionArgs)
	if !ok {
		return nil, fmt.Errorf("want args to be of type DistributionPolicyArgs, got %T", plArgs)
	}

	ctx := context.TODO()
	kosmosClient, err := getKosmosClient(ctx, &dcfg.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get kosmos-client, %v", err)
	}

	return &LeafNodeDistribution{
		kosmosClient: kosmosClient,
		handle:       fh,
	}, nil
}

func mapContains(in, out map[string]string) bool {
	if in == nil || out == nil {
		return false
	}

	for key, value := range in {
		if str, ok := out[key]; ok && str == value {
			return true
		}
	}

	return false
}

// isDaemonSetPod judges if this pod belongs to one daemonSet workload.
func isDaemonSetPod(pod *corev1.Pod) bool {
	for _, ownerRef := range pod.GetOwnerReferences() {
		if ownerRef.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func getDistribution(ctx context.Context, client *versioned.Clientset) (dp *kosmosv1alpha1.DistributionPolicy, err error) {
	distributionPolicy, err := client.KosmosV1alpha1().DistributionPolicies().Get(ctx, DistributionPolicy, metav1.GetOptions{})
	if err != nil {
		klog.ErrorS(err, "failed to get Distribution policy: "+DistributionPolicy)
		return nil, err
	}
	dp = distributionPolicy.DeepCopy()
	return
}

// Filter invoked at the filter extension point.
// Filter nodes based on distribution policy
func (knd *LeafNodeDistribution) Filter(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	// ignore daemonSet pod
	if isDaemonSetPod(pod) {
		return framework.NewStatus(framework.Success, "")
	}

	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	namespace := pod.Namespace
	if namespace == "" {
		namespace = corev1.NamespaceDefault
	}
	name := pod.Name

	dp, err := getDistribution(ctx, knd.kosmosClient)
	if err != nil {
		return framework.AsStatus(err)
	}

	resourceSelectors := dp.DistributionSpec.ResourceSelectors
	policies := dp.DistributionSpec.PolicyTerms
	var policyName string
	priority := NoneScope
	for _, selector := range resourceSelectors {
		matchNsLabel := false
		if selector.NamespaceLabelSelector != nil {
			nsLabel, err := knd.handle.ClientSet().CoreV1().Namespaces().Get(ctx, pod.Namespace, metav1.GetOptions{})
			if err != nil {
				return framework.AsStatus(err)
			}

			matchNsLabel = mapContains(selector.NamespaceLabelSelector.MatchLabels, nsLabel.GetLabels())
			if priority <= NamespaceLabelSelectorScope && matchNsLabel {
				policyName = selector.PolicyName
				priority = NamespaceLabelSelectorScope
			}
		}

		matchNsPrefix := false
		if selector.NamespacePrefix != "" {
			matchNsPrefix = strings.HasPrefix(namespace, selector.NamespacePrefix)
			if priority <= NamespacePrefixScope && matchNsPrefix {
				policyName = selector.PolicyName
				priority = NamespacePrefixScope
			}
		}

		matchNs := namespace == selector.Namespace
		if selector.Namespace != "" {
			if priority <= NamespaceScope && matchNs {
				policyName = selector.PolicyName
				priority = NamespaceScope
			}
		}

		inNamespace := matchNsLabel || matchNsPrefix || matchNs
		if selector.LabelSelector != nil {
			matchPodLabel := mapContains(selector.LabelSelector.MatchLabels, pod.GetLabels())
			if priority <= LabelSelectorScope && matchPodLabel {
				policyName = selector.PolicyName
				priority = LabelSelectorScope
			}

			if priority <= LabelSelectorInNsScope && inNamespace && matchPodLabel {
				policyName = selector.PolicyName
				priority = LabelSelectorInNsScope
			}
		}

		matchPodPrefix := false
		if selector.NamePrefix != "" {
			matchPodPrefix = strings.HasPrefix(name, selector.NamePrefix)
			if priority <= NamePrefixScope && matchPodPrefix {
				policyName = selector.PolicyName
				priority = NamePrefixScope
			}

			if priority <= NamePrefixInNsScope && inNamespace && matchPodPrefix {
				policyName = selector.PolicyName
				priority = NamePrefixInNsScope
			}
		}

		if selector.Name != "" {
			if priority <= NameScope && name == selector.Name {
				policyName = selector.PolicyName
				priority = NameScope
			}

			if priority <= NameInNsScope && inNamespace && name == selector.Name {
				policyName = selector.PolicyName
				priority = NameInNsScope
			}
		}
	}

	nodeType := kosmosv1alpha1.MIXNODE
	//var advancedTerm kosmosv1alpha1.AdvancedTerm
	if priority != NoneScope {
		for _, policy := range policies {
			if policyName == policy.Name {
				nodeType = policy.NodeType
				// TODO AdvancedTerm represents scheduling restrictions to a certain set of nodes.
				//advancedTerm = policy.AdvancedTerm
				break
			}
		}
	}

	nodeLabels := node.GetLabels()
	if nodeLabels == nil {
		nodeLabels = map[string]string{}
	}

	switch nodeType {
	case kosmosv1alpha1.HOSTNODE:
		{
			if _, ok := nodeLabels[KosmosNodeLabel]; ok {
				msg := fmt.Sprintf("This is a leaf node (%v) that does not match the distribution policy (%v) (host node)", node.Name, policyName)
				klog.V(4).Infof(msg)
				return framework.NewStatus(framework.UnschedulableAndUnresolvable, msg)
			} else {
				// TODO AdvancedTerm represents scheduling restrictions to a certain set of nodes.
				return framework.NewStatus(framework.Success, "")
			}
		}
	case kosmosv1alpha1.LEAFNODE:
		{
			if _, ok := nodeLabels[KosmosNodeLabel]; ok {
				// TODO AdvancedTerm represents scheduling restrictions to a certain set of nodes.
				return framework.NewStatus(framework.Success, "")
			} else {
				msg := fmt.Sprintf("This is a host node (%v) that does not match the distribution policy (%v) (leaf node)", node.Name, policyName)
				klog.V(4).Infof(msg)
				return framework.NewStatus(framework.UnschedulableAndUnresolvable, msg)
			}
		}
	case kosmosv1alpha1.MIXNODE:
		fallthrough
	default:
		{
			// TODO AdvancedTerm represents scheduling restrictions to a certain set of nodes.
			return framework.NewStatus(framework.Success, "")
		}
	}
}

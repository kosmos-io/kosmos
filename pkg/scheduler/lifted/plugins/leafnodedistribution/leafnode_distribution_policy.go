package leafnodedistribution

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/kosmos.io/kosmos/pkg/apis/config"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	dpInformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	dpLister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/helpers"
)

var _ framework.FilterPlugin = &LeafNodeDistribution{}

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "LeafNodeDistribution"

	KosmosNodeLabel = "kosmos.io/node"
)

const (
	NoneScope int = iota
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
	dpLister     *dpLister.DistributionPolicyLister
	cdpLister    *dpLister.ClusterDistributionPolicyLister
}

// Name returns name of the plugin.
func (lnd *LeafNodeDistribution) Name() string {
	return Name
}

func initDistributionInformer(kubeConfigPath *string) (*versioned.Clientset, *dpLister.DistributionPolicyLister, *dpLister.ClusterDistributionPolicyLister, error) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", *kubeConfigPath)
	if err != nil {
		klog.ErrorS(err, "Cannot create kubeconfig", "kubeConfigPath", *kubeConfigPath)
		return nil, nil, nil, err
	}

	client, err := versioned.NewForConfig(kubeConfig)
	if err != nil {
		klog.ErrorS(err, "cannot create client for LeafNodeDistribution", "kubeConfig", kubeConfig)
		return nil, nil, nil, err
	}

	dpInformerFactory := dpInformer.NewSharedInformerFactory(client, 0)
	dpInformer := dpInformerFactory.Kosmos().V1alpha1().DistributionPolicies()
	cdpInformer := dpInformerFactory.Kosmos().V1alpha1().ClusterDistributionPolicies()
	dpLister := dpInformer.Lister()
	cdpLister := cdpInformer.Lister()

	klog.V(5).InfoS("Start distributionInformer")
	ctx := context.Background()
	dpInformerFactory.Start(ctx.Done())
	dpInformerFactory.WaitForCacheSync(ctx.Done())

	return client, &dpLister, &cdpLister, nil
}

// New initializes a new plugin and returns it.
func New(plArgs runtime.Object, fh framework.Handle) (framework.Plugin, error) {
	klog.V(5).InfoS("Creating new LeafNodeDistribution plugin")
	dcfg, ok := plArgs.(*config.LeafNodeDistributionArgs)
	if !ok {
		return nil, fmt.Errorf("want args to be of type DistributionPolicyArgs, got %T", plArgs)
	}

	client, dpLister, cdpLister, err := initDistributionInformer(&dcfg.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get kosmos-client, %v", err)
	}

	return &LeafNodeDistribution{
		kosmosClient: client,
		dpLister:     dpLister,
		cdpLister:    cdpLister,
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

func getDistribution(namespace string, dpLister *dpLister.DistributionPolicyLister, cdpLister *dpLister.ClusterDistributionPolicyLister) (dps []*kosmosv1alpha1.DistributionPolicy, cdps []*kosmosv1alpha1.ClusterDistributionPolicy, err error) {
	dps, err = (*dpLister).DistributionPolicies(namespace).List(labels.Everything())
	if err != nil {
		klog.V(5).ErrorS(err, "Cannot get DistributionPolicies from DistributionPolicyLister")
		return nil, nil, err
	}

	cdps, err = (*cdpLister).List(labels.Everything())
	if err != nil {
		klog.V(5).ErrorS(err, "Cannot get ClusterDistributionPolicy from ClusterDistributionPolicyLister")
		return nil, nil, err
	}
	return
}

func matchDistributionPolicy(pod *corev1.Pod, dp *kosmosv1alpha1.DistributionPolicy) (policyName string, priority int, policies []kosmosv1alpha1.PolicyTerm) {
	name := pod.Name
	resourceSelectors := dp.DistributionSpec.ResourceSelectors
	for _, selector := range resourceSelectors {
		if pod.GetNamespace() != dp.Namespace {
			continue
		}

		if selector.LabelSelector != nil {
			matchPodLabel := mapContains(selector.LabelSelector.MatchLabels, pod.GetLabels())
			if priority <= LabelSelectorInNsScope && matchPodLabel {
				policyName = selector.PolicyName
				priority = LabelSelectorInNsScope
				policies = dp.DistributionSpec.PolicyTerms
			}
		}

		if selector.NamePrefix != "" {
			matchPodPrefix := strings.HasPrefix(name, selector.NamePrefix)
			if priority <= NamePrefixInNsScope && matchPodPrefix {
				policyName = selector.PolicyName
				priority = NamePrefixInNsScope
				policies = dp.DistributionSpec.PolicyTerms
			}
		}

		if selector.Name != "" {
			if priority <= NameInNsScope && name == selector.Name {
				policyName = selector.PolicyName
				priority = NameInNsScope
				policies = dp.DistributionSpec.PolicyTerms
			}
		}
	}

	return
}

func matchClusterDistributionPolicy(pod *corev1.Pod, cdp *kosmosv1alpha1.ClusterDistributionPolicy) (policyName string, priority int, policies []kosmosv1alpha1.PolicyTerm) {
	name := pod.Name
	resourceSelectors := cdp.DistributionSpec.ResourceSelectors
	for _, selector := range resourceSelectors {
		if selector.LabelSelector != nil {
			matchPodLabel := mapContains(selector.LabelSelector.MatchLabels, pod.GetLabels())
			if priority <= LabelSelectorScope && matchPodLabel {
				policyName = selector.PolicyName
				priority = LabelSelectorScope
				policies = cdp.DistributionSpec.PolicyTerms
			}
		}

		if selector.NamePrefix != "" {
			matchPodPrefix := strings.HasPrefix(name, selector.NamePrefix)
			if priority <= NamePrefixScope && matchPodPrefix {
				policyName = selector.PolicyName
				priority = NamePrefixScope
				policies = cdp.DistributionSpec.PolicyTerms
			}
		}

		if selector.Name != "" {
			if priority <= NameScope && name == selector.Name {
				policyName = selector.PolicyName
				priority = NameScope
				policies = cdp.DistributionSpec.PolicyTerms
			}
		}
	}

	return
}

func isTolerationTaints(taints []corev1.Taint, toleration *corev1.Toleration) bool {
	for _, taint := range taints {
		if taint.Key == toleration.Key && taint.Value == toleration.Value && taint.Effect == toleration.Effect {
			return true
		}
	}
	return false
}

func isToleration(node *corev1.Node, tolerations []*corev1.Toleration) (rs bool) {
	if len(tolerations) > 0 {
		for _, toleration := range tolerations {
			rs = isTolerationTaints(node.Spec.Taints, toleration)
			if rs {
				break
			}
		}
	}
	return false
}

// Filter invoked at the filter extension point.
// Filter nodes based on distribution policy
func (lnd *LeafNodeDistribution) Filter(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	// ignore daemonSet pod
	if helpers.IsDaemonSetPod(pod) {
		return framework.NewStatus(framework.Success, "")
	}

	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	dps, cdps, err := getDistribution(pod.GetNamespace(), lnd.dpLister, lnd.cdpLister)
	if err != nil {
		return framework.AsStatus(err)
	}

	var policyName string
	priority := NoneScope
	var policies []kosmosv1alpha1.PolicyTerm
	if len(dps) > 0 {
		for _, dp := range dps {
			policyName, priority, policies = matchDistributionPolicy(pod, dp)
		}
	}

	if priority == NoneScope && len(cdps) > 0 {
		for _, cdp := range cdps {
			policyName, priority, policies = matchClusterDistributionPolicy(pod, cdp)
		}
	}

	nodeType := kosmosv1alpha1.MIXNODE
	var advTerm kosmosv1alpha1.AdvancedTerm
	if priority != NoneScope {
		for _, policy := range policies {
			if policyName == policy.Name {
				nodeType = policy.NodeType
				// AdvancedTerm represents scheduling restrictions to a certain set of nodes.
				advTerm = policy.AdvancedTerm
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
				return framework.NewStatus(framework.Success, "")
			}
		}
	case kosmosv1alpha1.LEAFNODE:
		{
			if _, ok := nodeLabels[KosmosNodeLabel]; ok {
				return framework.NewStatus(framework.Success, "")
			} else {
				msg := fmt.Sprintf("This is a host node (%v) that does not match the distribution policy (%v) (leaf node)", node.Name, policyName)
				klog.V(4).Infof(msg)
				return framework.NewStatus(framework.UnschedulableAndUnresolvable, msg)
			}
		}
	case kosmosv1alpha1.ADVNODE:
		{
			selector := labels.Set(advTerm.NodeSelector).AsSelector()
			rs := (advTerm.NodeName == node.Name) || (advTerm.NodeSelector != nil && selector.Matches(labels.Set(node.Labels))) || isToleration(node, advTerm.Tolerations)
			if rs {
				return framework.NewStatus(framework.Success, "")
			} else {
				msg := fmt.Sprintf("This is node (%v) that does not match the distribution policy (%v)  AdvancedTerm ", node.Name, policyName)
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

// EventsToRegister returns the possible events that may make a failed Pod schedulable
func (lnd *LeafNodeDistribution) EventsToRegister() []framework.ClusterEvent {
	return []framework.ClusterEvent{
		{Resource: framework.Pod, ActionType: framework.All},
		{Resource: framework.Node, ActionType: framework.Add | framework.Delete | framework.UpdateNodeLabel},
	}
}

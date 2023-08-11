package floaterclient

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/floaterclient/command"
	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/initmaster/ctlmaster"
	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/util/apiclient"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/utils"
	cmdutil "github.com/kosmos.io/clusterlink/pkg/operator/util"
)

type FloaterInfo struct {
	NodeName string
	NodeIPs  []string

	PodName string
	PodIPs  []string
}

func (f *FloaterInfo) String() string {
	return fmt.Sprintf("nodeName: %s, nodeIPs: %s, podName: %s, podIPs: %s", f.NodeName, f.NodeIPs, f.PodName, f.PodIPs)
}

type Floater struct {
	KubeConfig        string
	Namespace         string
	ImageRepository   string
	Version           string
	DaemonSetName     string //"clusterlink-floater",
	PodWaitTime       int
	Port              string
	EnableHostNetwork bool `default:"false"`

	CIDRsMap map[string]string

	KubeClientSet  kubernetes.Interface
	KueResetConfig *rest.Config
}

// InitKubeClient Initialize a kubernetes client
func (i *Floater) InitKubeClient() error {
	if i.KubeConfig != "" {
		_, normalClient, _, err := apiclient.CreateKubeClient(i.KubeConfig)
		if err != nil {
			return err
		}
		i.KubeClientSet = normalClient

		restConfig, err := apiclient.RestConfig("", i.KubeConfig)
		if err != nil {
			return err
		}

		i.KueResetConfig = restConfig

	}
	return nil
}

// RunInit Deploy clusterlink in kubernetes
func (i *Floater) RunInit() error {

	i.DaemonSetName = "clusterlink-floater"

	// Create ns
	klog.Infof("Create namespace %s", i.Namespace)
	if err := cmdutil.CreateOrUpdateNamespace(i.KubeClientSet, cmdutil.NewNamespace(i.Namespace)); err != nil {
		return fmt.Errorf("create namespace %s failed: %v", i.Namespace, err)
	}
	// install RBAC
	if err := i.initFloaterRBAC(); err != nil {
		return err
	}

	if err := i.initFloaterDaemonSet(); err != nil {
		return err
	}

	return nil
}

func (i *Floater) applyServiceAccount() error {
	clFloaterServiceAccountBytes, err := utils.ParseTemplate(clusterlinkFloaterServiceAccount, RBACReplace{
		Namespace: i.Namespace,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink floater serviceaccount template :%v", err)
	}

	if clFloaterServiceAccountBytes == nil {
		return fmt.Errorf("wait clusterlink floater serviceaccount timeout")
	}

	clFloaterServiceAccount := &corev1.ServiceAccount{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clFloaterServiceAccountBytes, clFloaterServiceAccount); err != nil {
		return fmt.Errorf("decode floater serviceaccount error: %v", err)
	}

	if err := cmdutil.CreateOrUpdateServiceAccount(i.KubeClientSet, clFloaterServiceAccount); err != nil {
		return fmt.Errorf("create clusterlink floater serviceaccount error: %v", err)
	}

	return nil
}

func (i *Floater) applyClusterRole() error {
	clFloaterClusterRoleBytes, err := utils.ParseTemplate(clusterlinkClusterRole, RBACReplace{
		Namespace: i.Namespace,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink floater clusterrole template :%v", err)
	}

	if clFloaterClusterRoleBytes == nil {
		return fmt.Errorf("wait clusterlink floater clusterrole timeout")
	}

	clFloaterClusterRole := &rbacv1.ClusterRole{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clFloaterClusterRoleBytes, clFloaterClusterRole); err != nil {
		return fmt.Errorf("decode floater clusterrole error: %v", err)
	}

	if err := cmdutil.CreateOrUpdateClusterRole(i.KubeClientSet, clFloaterClusterRole); err != nil {
		return fmt.Errorf("create clusterlink floater clusterrole error: %v", err)
	}

	return nil
}

func (i *Floater) applyClusterRoleBinding() error {
	clFloaterClusterRoleBindingBytes, err := utils.ParseTemplate(clusterlinkClusterRoleBinding, RBACReplace{
		Namespace: i.Namespace,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink floater clusterrolebinding template :%v", err)
	}

	if clFloaterClusterRoleBindingBytes == nil {
		return fmt.Errorf("wait clusterlink floater clusterrolebinding timeout")
	}

	clFloaterClusterRoleBinding := &rbacv1.ClusterRoleBinding{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clFloaterClusterRoleBindingBytes, clFloaterClusterRoleBinding); err != nil {
		return fmt.Errorf("decode floater clusterrolebinding error: %v", err)
	}

	if err := cmdutil.CreateOrUpdateClusterRoleBinding(i.KubeClientSet, clFloaterClusterRoleBinding); err != nil {
		return fmt.Errorf("create clusterlink floater clusterrolebinding error: %v", err)
	}

	return nil
}

func (i *Floater) initFloaterRBAC() error {

	klog.Info("Create Clusterlink RBAC")

	err := i.applyClusterRole()
	if err != nil {
		return err
	}

	err = i.applyClusterRoleBinding()
	if err != nil {
		return err
	}

	err = i.applyServiceAccount()
	if err != nil {
		return err
	}

	return nil

}

func (i *Floater) initFloaterDaemonSet() error {
	replace := DaemonSetReplace{
		Namespace:       i.Namespace,
		Version:         i.Version,
		ImageRepository: i.ImageRepository,
		DaemonSetName:   i.DaemonSetName,
		Port:            i.Port,
	}
	if i.EnableHostNetwork {
		replace.EnableHostNetwork = i.EnableHostNetwork
	}

	klog.Infof("Create Clusterlink %s DaemonSet, version: %s", "clusterlink-floater", i.Version)

	clusterlinkFloaterDaemonSetBytes, err := utils.ParseTemplate(clusterlinkFloaterDaemonSet, replace)

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink floater daemonset template :%v", err)
	}

	if clusterlinkFloaterDaemonSetBytes == nil {
		return fmt.Errorf("wait clusterlink floater daemonset timeout")
	}

	clFloaterDaemonSet := &appsv1.DaemonSet{}

	if err = kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clusterlinkFloaterDaemonSetBytes, clFloaterDaemonSet); err != nil {
		return fmt.Errorf("decode floater daemonset error: %v", err)
	}

	if err = cmdutil.CreateOrUpdateDaemonSet(i.KubeClientSet, clFloaterDaemonSet); err != nil {
		return fmt.Errorf("create clusterlink floater daemonset error: %v", err)
	}

	daemonSetLabel := map[string]string{"app": replace.DaemonSetName}
	if err = ctlmaster.WaitPodReady(i.KubeClientSet, i.Namespace, ctlmaster.MapToString(daemonSetLabel), i.PodWaitTime); err != nil {
		return err
	}

	return nil
}

func (i *Floater) GetPodInfo() ([]*FloaterInfo, error) {
	selector := ctlmaster.MapToString(map[string]string{"app": i.DaemonSetName})
	pods, err := i.KubeClientSet.CoreV1().Pods(i.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods in %s with selector %s", i.Namespace, selector)
	}

	var floaterInfos []*FloaterInfo
	for _, pod := range pods.Items {
		podInfo := &FloaterInfo{
			NodeName: pod.Spec.NodeName,
			PodName:  pod.GetObjectMeta().GetName(),
			PodIPs:   PodIPToArray(pod.Status.PodIPs),
		}

		floaterInfos = append(floaterInfos, podInfo)
	}

	return floaterInfos, nil
}

func PodIPToArray(podIPs []corev1.PodIP) []string {
	var ret []string

	for _, podIP := range podIPs {
		ret = append(ret, podIP.IP)
	}

	return ret
}

func (i *Floater) GetNodesInfo() ([]*FloaterInfo, error) {
	selector := ctlmaster.MapToString(map[string]string{"app": i.DaemonSetName})
	pods, err := i.KubeClientSet.CoreV1().Pods(i.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods in %s with selector %s", i.Namespace, selector)
	}

	nodes, err := i.KubeClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(nodes.Items) == 0 {
		return nil, fmt.Errorf("unable to list any node")
	}

	var floaterInfos []*FloaterInfo
	for _, pod := range pods.Items {
		for _, node := range nodes.Items {
			if pod.Spec.NodeName == node.Name {
				nodeInfo := &FloaterInfo{
					NodeName: node.Name,
					NodeIPs:  NodeIPToArray(node),
					PodName:  pod.Name,
				}
				floaterInfos = append(floaterInfos, nodeInfo)
			}
		}
	}

	return floaterInfos, nil
}

func NodeIPToArray(node corev1.Node) []string {
	var nodeIPs []string

	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			nodeIPs = append(nodeIPs, addr.Address)
		}
	}

	return nodeIPs
}

func (i *Floater) CommandExec(floaterInfo *FloaterInfo, cmd command.Command) *command.Result {
	req := i.KubeClientSet.CoreV1().RESTClient().Post().Resource("pods").Namespace(i.Namespace).Name(floaterInfo.PodName).
		SubResource("exec").
		Param("container", "floater").
		Param("command", "/bin/sh").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false")

	outBuffer := &bytes.Buffer{}
	errBuffer := &bytes.Buffer{}

	// restConfig := i.KueResetConfig
	// var err error
	// if i.KubeConfig != "" {
	// 	restConfig, err = apiclient.RestConfig("", i.KubeConfig)
	// 	if err != nil {
	// 		return command.ParseError(err)
	// 	}
	// }

	exec, err := remotecommand.NewSPDYExecutor(i.KueResetConfig, "POST", req.URL())
	if err != nil {
		return command.ParseError(err)
	}

	// timeout 5s
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmdStr := cmd.GetCommandStr()

	klog.Infof("cmdStr: %s", cmdStr)
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  strings.NewReader(cmdStr),
		Stdout: outBuffer,
		Stderr: errBuffer,
		Tty:    false,
	})

	if err != nil {
		klog.Infof("error: %s", err)
		return command.ParseError(fmt.Errorf("%s, stderr: %s", err, errBuffer.String()))
	}

	return cmd.ParseResult(outBuffer.String())
}

package framework

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/hack/projectpath"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

func FetchClusters(client versioned.Interface) ([]kosmosv1alpha1.Cluster, error) {
	clusters, err := client.KosmosV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return clusters.DeepCopy().Items, nil
}

func CreateClusters(client versioned.Interface, cluster *kosmosv1alpha1.Cluster) (err error) {
	_, err = client.KosmosV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func DeleteClusters(client versioned.Interface, clustername string) (err error) {
	err = client.KosmosV1alpha1().Clusters().Delete(context.TODO(), clustername, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func FetchNodes(client kubernetes.Interface) ([]corev1.Node, error) {
	nodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nodes.DeepCopy().Items, nil
}
func DeleteNode(client kubernetes.Interface, node string) (err error) {
	err = client.CoreV1().Nodes().Delete(context.TODO(), node, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func UpdateNodeLabels(client kubernetes.Interface, node corev1.Node) (err error) {
	_, err = client.CoreV1().Nodes().Update(context.TODO(), &node, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func WaitNodePresentOnCluster(client kubernetes.Interface, node string) {
	ginkgo.By(fmt.Sprintf("Waiting for node(%v) on host cluster", node), func() {
		gomega.Eventually(func() bool {
			_, err := client.CoreV1().Nodes().Get(context.TODO(), node, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Failed to get node(%v) on host cluster", node, err)
				return false
			}
			return true
		}, PollTimeout, PollInterval).Should(gomega.Equal(true))
	})
}

func LoadRESTClientConfig(kubeconfig string, context string) (*rest.Config, error) {
	loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	loadedConfig, err := loader.Load()
	if err != nil {
		return nil, err
	}

	if context == "" {
		context = loadedConfig.CurrentContext
	}
	klog.Infof("Use context %v", context)

	return clientcmd.NewNonInteractiveClientConfig(
		*loadedConfig,
		context,
		&clientcmd.ConfigOverrides{},
		loader,
	).ClientConfig()
}

func CreateCluster(clusterName, podCIDR, serviceCIDR string) error {
	command := fmt.Sprintf("source %s/hack/cluster.sh && create_cluster %s %s %s", projectpath.Root, clusterName, podCIDR, serviceCIDR)
	err := runCmd("bash", "-c", command)
	return err
}

func Join(hostCluster, memberCluster string) error {
	command := fmt.Sprintf("source %s/hack/cluster.sh && join_cluster %s %s", projectpath.Root, hostCluster, memberCluster)
	err := runCmd("bash", "-c", command)
	return err
}

func DeleteCluster(memberCluster string) error {
	command := fmt.Sprintf("source %s/hack/cluster.sh && delete_cluster %s", projectpath.Root, memberCluster)
	err := runCmd("bash", "-c", command)
	return err
}

func LoadClusterLink(memberCluster string) error {
	command := fmt.Sprintf("source %s/hack/cluster.sh && load_clusterlink_images %s", projectpath.Root, memberCluster)
	err := runCmd("bash", "-c", command)
	return err
}

func runCmd(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	stdoutIn, err := cmd.StdoutPipe()
	if err != nil {
		klog.Errorf("runCmd get stdout err %v", err)
		return err
	}
	stderrIn, err := cmd.StderrPipe()
	if err != nil {
		klog.Errorf("runCmd get stderr err %v", err)
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	// read from stdout and stderr in background and log any output
	go func() {
		scannerOut := bufio.NewScanner(stdoutIn)
		for scannerOut.Scan() {
			log.Println(scannerOut.Text())
		}
	}()
	go func() {
		scannerErr := bufio.NewScanner(stderrIn)
		for scannerErr.Scan() {
			log.Println(scannerErr.Text())
		}
	}()

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}

package framework

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/clusterlink/projectpath"
)

func FetchClusters(client versioned.Interface) ([]clusterlinkv1alpha1.Cluster, error) {
	clusters, err := client.ClusterlinkV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return clusters.Items, nil
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

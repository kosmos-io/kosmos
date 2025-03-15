package cert

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func BackDir(vc *v1alpha1.VirtualCluster) string {
	// back dir
	return fmt.Sprintf("backup-%s-%s", vc.GetNamespace(), vc.GetName())
}

func SaveRuntimeObjectToYAML(obj runtime.Object, fileName, dirName string) error {
	scheme := runtime.NewScheme()
	serializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: true})

	filePath := filepath.Join(dirName, fmt.Sprintf("%s.yaml", fileName))
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	err = serializer.Encode(obj, file)
	if err != nil {
		return fmt.Errorf("failed to serialize secret to YAML: %w", err)
	}

	return nil
}

// nolint
func SaveStringToDir(data string, fileName, dirName string) error {
	backupFilePath := filepath.Join(dirName, fileName)
	err := os.WriteFile(backupFilePath, []byte(data), 0644)
	if err != nil {
		return fmt.Errorf("write backup file failed: %s", err.Error())
	}

	return err
}

func HostClusterConfigPath() string {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if len(kubeconfigPath) == 0 {
		return "~/.kube/config"
	}
	return kubeconfigPath
}

func runKubectlCommand(args ...string) error {
	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec cmd failed: %s\n err: %v", string(output), err)
	}
	fmt.Println(string(output))
	return nil
}

func WaitPodReady(k8sClient kubernetes.Interface, namespace string) error {
	timeout := constants.WaitCorePodsRunningTimeout

	endTime := time.Now().Second() + int(timeout.Seconds())

	startTime := time.Now().Second()
	if startTime > endTime {
		return errors.New("Timeout waiting for all pods running")
	}
	klog.Infof("Check if all pods ready in namespace %s", namespace)
	err := wait.PollWithContext(context.TODO(), 5*time.Second, time.Duration(endTime-startTime)*time.Second, func(ctx context.Context) (done bool, err error) {
		klog.Infof("Check if virtualcluster %s all deployments ready in namespace %s", namespace)
		deployList, err := k8sClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, errors.Wrapf(err, "Get deployment list in namespace %s error", namespace)
		}
		for _, deploy := range deployList.Items {
			if deploy.Status.AvailableReplicas != deploy.Status.Replicas {
				klog.Infof("Deployment %s/%s is not ready yet. Available replicas: %d, Desired: %d. Waiting...", deploy.Name, namespace, deploy.Status.AvailableReplicas, deploy.Status.Replicas)
				return false, nil
			}
		}

		klog.Infof("Check if virtualcluster %s all statefulset ready in namespace %s", namespace)
		stsList, err := k8sClient.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, errors.Wrapf(err, "Get statefulset list in namespace %s error", namespace)
		}
		for _, sts := range stsList.Items {
			if sts.Status.AvailableReplicas != sts.Status.Replicas {
				klog.Infof("Statefulset %s/%s is not ready yet. Available replicas: %d, Desired: %d. Waiting...", sts.Name, namespace, sts.Status.AvailableReplicas, sts.Status.Replicas)
				return false, nil
			}
		}

		klog.Infof("Check if virtualcluster %s all daemonset ready in namespace %s", namespace)
		damonsetList, err := k8sClient.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, errors.Wrapf(err, "Get daemonset list in namespace %s error", namespace)
		}
		for _, daemonset := range damonsetList.Items {
			if daemonset.Status.CurrentNumberScheduled != daemonset.Status.NumberReady {
				klog.Infof("Daemonset %s/%s is not ready yet. Scheduled replicas: %d, Ready: %d. Waiting...", daemonset.Name, namespace, daemonset.Status.CurrentNumberScheduled, daemonset.Status.NumberReady)
				return false, nil
			}
		}

		return true, nil
	})
	return err
}

func WaitNodeReady(k8sClient kubernetes.Interface) error {
	// wait all node ready
	timeout := constants.WaitCorePodsRunningTimeout

	endTime := time.Now().Second() + int(timeout.Seconds())

	startTime := time.Now().Second()
	if startTime > endTime {
		return errors.New("Timeout waiting for all pods running")
	}
	klog.Infof("Check if all node ready")
	err := wait.PollWithContext(context.TODO(), 5*time.Second, time.Duration(endTime-startTime)*time.Second, func(ctx context.Context) (done bool, err error) {
		nodes, err := k8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, node := range nodes.Items {
			if !util.IsNodeReady(node.Status.Conditions) {
				return false, nil
			}
		}
		return true, nil
	})
	return err
}

func WaitDaemonsetReady(k8sClient kubernetes.Interface, namespace, name string) error {
	timeout := constants.WaitCorePodsRunningTimeout

	endTime := time.Now().Second() + int(timeout.Seconds())

	startTime := time.Now().Second()
	if startTime > endTime {
		return errors.New("Timeout waiting for all pods running")
	}
	klog.Infof("Check if all pods ready in namespace %s", namespace)
	err := wait.PollWithContext(context.TODO(), 5*time.Second, time.Duration(endTime-startTime)*time.Second, func(ctx context.Context) (done bool, err error) {
		klog.Infof("Check if virtualcluster %s all daemonset ready in namespace %s", namespace)
		damonsetList, err := k8sClient.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, errors.Wrapf(err, "Get daemonset list in namespace %s error", namespace)
		}
		for _, daemonset := range damonsetList.Items {
			if daemonset.GetName() != name {
				continue
			}
			if daemonset.Status.CurrentNumberScheduled != daemonset.Status.NumberReady {
				klog.Infof("Daemonset %s/%s is not ready yet. Scheduled replicas: %d, Ready: %d. Waiting...", daemonset.Name, namespace, daemonset.Status.CurrentNumberScheduled, daemonset.Status.NumberReady)
				return false, nil
			}
		}

		return true, nil
	})
	return err
}

func GetVirtualNodeIP(kosmosClient versioned.Interface, vc *v1alpha1.VirtualCluster) ([]string, error) {
	globalNodeList, err := kosmosClient.KosmosV1alpha1().GlobalNodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list global nodes: %w", err)
	}
	nodeIPs := []string{}
	nodeInfos := vc.Spec.PromoteResources.NodeInfos
	for _, node := range globalNodeList.Items {
		for _, nodeInfo := range nodeInfos {
			if node.GetName() == nodeInfo.NodeName {
				nodeIPs = append(nodeIPs, node.Spec.NodeIP)
			}
		}
	}
	return nodeIPs, nil
}

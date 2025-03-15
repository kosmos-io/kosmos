package cert

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func RunBackupSecrets(data *Option) error {
	klog.Infof("backup secrets for virtual cluster")
	// create dir
	vc := data.VirtualCluster()
	dirName := BackDir(vc)

	err := os.Mkdir(dirName, 0755)
	if err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("create backup dir failed: %s", err.Error())
		}
	}
	klog.InfoS("create backup dir successed:", dirName)

	// backup certs
	secrets := []string{
		util.GetEtcdCertName(vc.GetName()),
		util.GetCertName(vc.GetName()),
		util.GetAdminConfigSecretName(vc.GetName()),
		util.GetAdminConfigClusterIPSecretName(vc.GetName()),
	}

	for _, secret := range secrets {
		klog.InfoS("backup secret", "secret", secret)
		cert, err := data.RemoteClient().CoreV1().Secrets(vc.GetNamespace()).Get(context.TODO(), secret, metav1.GetOptions{})
		if err != nil {
			return err
		}

		err = SaveRuntimeObjectToYAML(cert, secret, dirName)

		if err != nil {
			return fmt.Errorf("write backup file failed: %s", err.Error())
		}

		klog.InfoS("backup secret successed", "file", secret)
	}

	return nil
}

func RunReCreateCertAndKubeConfig(data *Option) error {
	klog.Infof("update ca.crt and kubeconfig for virtual cluster")
	exec, err := controller.UpdateCertPhase(data.VirtualCluster(), data.Client, data.restConfig, &v1alpha1.KubeNestConfiguration{})
	if err != nil {
		return err
	}

	return exec.Execute()
}

func UpdateKubeProxyConfig(data *Option) error {
	klog.Infof("update kube-proxy config map")
	configCert, err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Get(context.TODO(), util.GetAdminConfigSecretName(data.GetName()), metav1.GetOptions{})
	if err != nil {
		return err
	}

	kubeconfigstring := string(configCert.Data[constants.KubeConfig])

	vc := data.VirtualCluster()

	k8sClient, err := util.GenerateKubeclient(vc)
	if err != nil {
		return err
	}

	kubeproxycm, err := k8sClient.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "kube-proxy", metav1.GetOptions{})
	if err != nil {
		return err
	}

	kubeproxycmkey := constants.KubeConfig + ".conf"

	kubeproxycm.Data[kubeproxycmkey] = kubeconfigstring

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		kubeproxycm.ResourceVersion = ""
		_, err = k8sClient.CoreV1().ConfigMaps("kube-system").Update(context.TODO(), kubeproxycm, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		return err
	}

	klog.Infof("save files to disk")
	// save to dir
	dirName := BackDir(vc)
	err = SaveStringToDir(kubeconfigstring, "kubeconfig.conf", dirName)

	if err != nil {
		return fmt.Errorf("write backup file failed: %s", err.Error())
	}
	// update VC
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentVC, err := data.KosmosClient().KosmosV1alpha1().VirtualClusters(vc.GetNamespace()).Get(context.TODO(), vc.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		currentVC.Spec.Kubeconfig = base64.StdEncoding.EncodeToString([]byte(kubeconfigstring))
		_, err = data.KosmosClient().KosmosV1alpha1().VirtualClusters(vc.GetNamespace()).Update(context.TODO(), currentVC, metav1.UpdateOptions{})
		// nessary to update the cache
		data.UpdateVirtualCluster(currentVC)
		return err
	})

	// get ca.crt
	cert, err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Get(context.TODO(), util.GetCertName(data.GetName()), metav1.GetOptions{})
	if err != nil {
		return err
	}

	cacrt := cert.Data[constants.CaCertAndKeyName+".crt"]

	err = SaveStringToDir(string(cacrt), "ca.crt", dirName)
	if err != nil {
		return fmt.Errorf("write backup file failed: %s", err.Error())
	}

	return nil
}

func RestartVirtualControlPlanePod(data *Option) error {
	klog.Infof("restart control-plane pod in host cluster")
	vc := data.VirtualCluster()

	namespace := vc.GetNamespace()
	name := vc.GetName()
	commands := [][]string{
		{
			"--kubeconfig",
			HostClusterConfigPath(),
			"-n", namespace,
			"rollout",
			"restart",
			fmt.Sprintf("statefulset.apps/%s-etcd", name),
			fmt.Sprintf("deployment.apps/%s-apiserver", name),
			fmt.Sprintf("deployment.apps/%s-kube-controller-manager", name),
			fmt.Sprintf("deployment.apps/%s-virtualcluster-scheduler", name),
			fmt.Sprintf("deployment.apps/%s-coredns", name),
		},
	}

	for _, args := range commands {
		klog.InfoS("run command:", strings.Join(args, " "))
		if err := runKubectlCommand(args...); err != nil {
			// Sometimes an exception occurs: Error from server (NotFound): deployments.apps "example0318-coredns" not found.
			// This is not a problem, because the coredns is deployed in the virtual cluster.
			klog.InfoS("run command failed:", err)
		}
	}

	// wait for pod ready
	return WaitPodReady(data.RemoteClient(), namespace)
}

func RestartVirtualPod(data *Option) error {
	klog.Infof("restart pod in virtual cluster")
	vc := data.VirtualCluster()

	dirName := BackDir(vc)
	commands := [][]string{
		{
			"--kubeconfig",
			fmt.Sprintf("./%s/kubeconfig.conf", dirName),
			"-n",
			"kube-system",
			"rollout",
			"restart",
			"deployment.apps/calico-typha",
			"deployment.apps/calico-kube-controllers",
			"daemonset.apps/calico-node",
			"daemonset.apps/kube-proxy",
			"daemonset.apps/konnectivity-agent",
		},
	}

	for _, args := range commands {
		klog.InfoS("run command:", strings.Join(args, " "))
		if err := runKubectlCommand(args...); err != nil {
			klog.InfoS("run command failed:", err)
		}
	}

	k8sClient, err := util.GenerateKubeclient(vc)
	if err != nil {
		return err
	}

	return WaitDaemonsetReady(k8sClient, "kube-system", "konnectivity-agent")
}

func RestartVirtualWorkerKubelet(data *Option) error {
	vc := data.VirtualCluster()

	klog.Infof("get ips of node-agent")
	nodeIPs, err := GetVirtualNodeIP(data.KosmosClient(), vc)
	if err != nil {
		return err
	}

	klog.Infof("send ca.crt and kubelet.conf to node and run cert.sh")
	for _, nodeIP := range nodeIPs {
		// back dir
		dirName := BackDir(vc)

		exectHelper := exector.NewExectorHelper(nodeIP, "")

		// upload shell
		scpShellCmd := &exector.SCPExector{
			DstFilePath: ".",
			DstFileName: "cert.sh",
			SrcByte:     []byte(certShell),
		}

		ret := exectHelper.DoExector(context.TODO().Done(), scpShellCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("scp shell to node %s failed: %s", nodeIP, ret.String())
		}

		// upload ca.crt
		scpCrtCmd := &exector.SCPExector{
			DstFilePath: "/apps/conf/kosmos/cert/",
			DstFileName: "ca.crt",
			SrcFile:     fmt.Sprintf("./%s/%s", dirName, "ca.crt"),
		}

		ret = exectHelper.DoExector(context.TODO().Done(), scpCrtCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("scp ca.crt to node %s failed: %s", nodeIP, ret.String())
		}

		// upload kubeconfig
		scpKubeconfiCmd := &exector.SCPExector{
			DstFilePath: "/apps/conf/kosmos/cert/",
			DstFileName: "kubelet.conf",
			SrcFile:     fmt.Sprintf("./%s/%s", dirName, "kubeconfig.conf"),
		}

		ret = exectHelper.DoExector(context.TODO().Done(), scpKubeconfiCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("scp kubeconfi to node %s failed: %s", nodeIP, ret.String())
		}

		updateCmd := &exector.CMDExector{
			Cmd: "bash cert.sh update",
		}

		ret = exectHelper.DoExector(context.TODO().Done(), updateCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("do update shell on node %s failed: %s", nodeIP, ret.String())
		}
	}

	k8sClient, err := util.GenerateKubeclient(vc)
	if err != nil {
		return err
	}

	return WaitNodeReady(k8sClient)
}

func RunCheckEnvironment(data *Option) error {
	klog.Infof("try to connect to node-agent")
	vc := data.VirtualCluster()
	nodeIPs, err := GetVirtualNodeIP(data.KosmosClient(), vc)
	if err != nil {
		return err
	}
	for _, nodeIP := range nodeIPs {
		exectHelper := exector.NewExectorHelper(nodeIP, "")

		testCmd := &exector.CMDExector{
			Cmd: "pwd",
		}

		ret := exectHelper.DoExector(context.TODO().Done(), testCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("do check on node %s failed: %s", nodeIP, ret.String())
		}
	}
	klog.Infof("try to run command kubectl")
	namespace := vc.GetNamespace()
	name := vc.GetName()
	commands := [][]string{
		{
			"--kubeconfig",
			HostClusterConfigPath(),
			"-n", namespace,
			"get",
			"vc",
			name,
		},
	}

	for _, args := range commands {
		klog.InfoS("run command:", strings.Join(args, " "))
		if err := runKubectlCommand(args...); err != nil {
			klog.InfoS("run command failed:", err)
			return err
		}
	}

	return nil
}

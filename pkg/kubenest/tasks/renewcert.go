package tasks

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewRenewCertsTask() workflow.Task {
	return workflow.Task{
		Name: "Renew-Certs",
		Run:  runRenewCerts,
	}
}

func UpdateKubeProxyConfig(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("renew-certs task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[renew-certs] Running renew-certs task", "virtual cluster", klog.KObj(data))

	configCert, err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Get(context.TODO(), util.GetAdminConfigSecretName(data.GetName()), metav1.GetOptions{})
	if err != nil {
		return err
	}

	kubeconfigstring := string(configCert.Data[constants.KubeConfig])

	kubeproxycm, err := data.RemoteClient().CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "kube-proxy", metav1.GetOptions{})
	if err != nil {
		return err
	}

	kubeproxycmkey := constants.KubeConfig + ".conf"

	kubeproxycm.Data[kubeproxycmkey] = kubeconfigstring

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		kubeproxycm.ResourceVersion = ""
		_, err = data.RemoteClient().CoreV1().ConfigMaps("kube-system").Update(context.TODO(), kubeproxycm, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		return err
	}

	// save to dir
	vc := data.VirtualCluster()
	dirName := fmt.Sprintf("backup-%s-%s", vc.GetNamespace(), vc.GetName())
	err = SaveStringToDir(kubeconfigstring, "kubeconfig.conf", dirName)

	if err != nil {
		return fmt.Errorf("write backup file failed: %s", err.Error())
	}

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

func runRenewCerts(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("renew-certs task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[renew-certs] Running renew-certs task", "virtual cluster", klog.KObj(data))

	// update kube-proxy cm kubeconfig
	if err := UpdateKubeProxyConfig(r); err != nil {
		return err
	}
	// restart core-dns apiserver kube-controller kube-scheduler
	// update kubelet config  and restart
	// restart calico  kube-proxy

	return nil
}

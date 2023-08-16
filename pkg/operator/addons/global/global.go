package global

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/operator/addons/option"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/utils"
	cmdutil "github.com/kosmos.io/clusterlink/pkg/operator/util"
)

type Installer struct {
}

func New() *Installer {
	return &Installer{}
}

func (i *Installer) Install(opt *option.AddonOption) error {
	clNamespaceBytes, err := utils.ParseTemplate(clusterlinkNamespace, NamespaceReplace{
		Namespace: opt.GetSpecNamespace(),
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink namespace template :%v", err)
	}

	if clNamespaceBytes == nil {
		return fmt.Errorf("wait klusterlink namespace timeout")
	}

	clNamespace := &corev1.Namespace{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clNamespaceBytes, clNamespace); err != nil {
		return fmt.Errorf("decode namespace error: %v", err)
	}

	if err := cmdutil.CreateOrUpdateNamespace(opt.KubeClientSet, clNamespace); err != nil {
		return fmt.Errorf("create clusterlink namespace error: %v", err)
	}

	// TODO: wati
	klog.Infof("Install clusterlink namespace on cluster successfully")
	return nil
}

// Uninstall resources related to CR:cluster
func (i *Installer) Uninstall(opt *option.AddonOption) error {
	klog.Infof("Don't remove clusterlink namespace on cluster for test")
	// nsClient := opt.KubeClientSet.CoreV1().Namespaces()
	// if err := nsClient.Delete(context.TODO(), opt.GetSpecNamespace(), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
	// 	return err
	// }

	// klog.Infof("Uninstall clusterlink namespace on cluster successfully")
	return nil
}

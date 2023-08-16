package ctlmaster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	apiclient "github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/util/apiclient"
)

func (i *CommandInitOption) initClusterlinkConfigmap() error {
	configmapName := "external-kubeconfig"
	klog.Info("Create configmap external-kubeconfig")

	configmapContent, err := ReadKubeconfigFile(apiclient.KubeConfigPath(i.KubeConfig))
	if err != nil {
		return err
	}

	configMapKubeconfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: i.Namespace,
		},
		Data: map[string]string{
			"kubeconfig": configmapContent,
		},
	}

	_, err = i.KubeClientSet.CoreV1().ConfigMaps(i.Namespace).Create(context.Background(),
		configMapKubeconfig, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			klog.Errorf("Create configmap %s error: %v", configmapName, err)
			return fmt.Errorf("create configmap %s error: %w", configmapName, err)
		}

		klog.Infof("ConfigMap %s exists,update it", configmapName)
		_, err := i.KubeClientSet.CoreV1().ConfigMaps(i.Namespace).Update(context.Background(),
			configMapKubeconfig, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Update configmap %s error: %v", configmapName, err)
			return fmt.Errorf("update configmap %s error: %w", configmapName, err)
		}
	}

	return nil
}

package controller

import (
	"context"
	"encoding/base64"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

type Executer struct {
	client.Client
	virtualCluster *v1alpha1.VirtualCluster
	phase          *workflow.Phase
	config         *rest.Config
}

func NewExecuter(virtualCluster *v1alpha1.VirtualCluster, c client.Client, config *rest.Config) (*Executer, error) {
	var phase *workflow.Phase

	action := recognizeActionFor(virtualCluster)
	switch action {
	case constants.InitAction:
		opts := []kubenest.InitOpt{
			kubenest.NewInitOptWithVirtualCluster(virtualCluster),
			kubenest.NewInitOptWithKubeconfig(config),
		}
		options := kubenest.NewPhaseInitOptions(opts...)
		phase = kubenest.NewInitPhase(options)
	case constants.DeInitAction:
		//TODO deinit
	default:
		return nil, fmt.Errorf("failed to recognize action for virtual cluster %s", virtualCluster.Name)
	}

	return &Executer{
		virtualCluster: virtualCluster,
		Client:         c,
		phase:          phase,
		config:         config,
	}, nil
}

func (e *Executer) Execute() error {
	klog.InfoS("Start execute the workflow", "workflow", "virtual cluster", klog.KObj(e.virtualCluster))

	if err := e.phase.Run(); err != nil {
		klog.ErrorS(err, "failed to executed the workflow", "workflow", "virtual cluster", klog.KObj(e.virtualCluster))
	}
	//TODO modify status for virtualcluster
	if err := e.afterRunPhase(); err != nil {
		return err
	}
	klog.InfoS("Successfully executed the workflow", "workflow", "virtual cluster", klog.KObj(e.virtualCluster))
	return nil
}

func (e *Executer) afterRunPhase() error {
	localClusterClient, err := clientset.NewForConfig(e.config)
	if err != nil {
		return fmt.Errorf("error when creating local cluster client, err: %w", err)
	}
	secret, err := localClusterClient.CoreV1().Secrets(e.virtualCluster.GetNamespace()).Get(context.TODO(), fmt.Sprintf("%s-%s", e.virtualCluster.GetName(), constants.AdminConfig), metav1.GetOptions{})
	if err != nil {
		return err
	}

	kubeconfigBytes, _ := secret.Data[constants.KubeConfig]
	configString := base64.StdEncoding.EncodeToString(kubeconfigBytes)
	e.virtualCluster.Spec.Kubeconfig = configString
	e.virtualCluster.Status.Phase = v1alpha1.Completed
	return e.Client.Update(context.TODO(), e.virtualCluster)
}

func recognizeActionFor(virtualCluster *v1alpha1.VirtualCluster) constants.Action {
	if !virtualCluster.DeletionTimestamp.IsZero() {
		return constants.DeInitAction
	}
	return constants.InitAction
}

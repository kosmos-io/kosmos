package cert

import (
	"context"
	"fmt"
	"reflect"
	"runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/scheme"
)

type Option struct {
	client.Client
	remoteClient   clientset.Interface
	kosmosClient   versioned.Interface
	virtualCluster *v1alpha1.VirtualCluster
	dynamicClient  *dynamic.DynamicClient
	restConfig     *rest.Config
}

func NewCertOption(o *RenewOptions) (*Option, error) {
	config, err := clientcmd.BuildConfigFromFlags("", o.KubeconfigPath)
	if err != nil {
		klog.Infof("Failed to build config: %v\n", err)
		return nil, err
	}

	cli, err := client.New(config, client.Options{Scheme: scheme.NewSchema()})
	if err != nil {
		klog.Infof("Failed to create client: %v\n", err)
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Infof("Failed to create dynamic client: %v\n", err)
		return nil, err
	}

	kosmosClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error when creating  kosmosClient client, err: %w", err)
	}

	localClusterClient, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error when creating local cluster client, err: %w", err)
	}

	var remoteClient clientset.Interface = localClusterClient

	gvr := schema.GroupVersionResource{
		Group:    "kosmos.io",
		Version:  "v1alpha1",
		Resource: "virtualclusters",
	}

	unstructuredObj, err := dynamicClient.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get CRD resources: %v\n", err)
		return nil, err
	}

	var virtualCluster v1alpha1.VirtualCluster
	err = k8sruntime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &virtualCluster)
	if err != nil {
		klog.Infof("Error converting to structured object: %v\n", err)
		return nil, err
	}

	return &Option{
		Client:         cli,
		remoteClient:   remoteClient,
		kosmosClient:   kosmosClient,
		virtualCluster: &virtualCluster,
		dynamicClient:  dynamicClient,
		restConfig:     config,
	}, nil
}

func (c *Option) GetName() string {
	return c.virtualCluster.GetName()
}

func (c *Option) GetNamespace() string {
	return c.virtualCluster.GetNamespace()
}

func (c *Option) VirtualCluster() *v1alpha1.VirtualCluster {
	return c.virtualCluster
}

func (c *Option) DynamicClient() *dynamic.DynamicClient {
	return c.dynamicClient
}

func (c *Option) RemoteClient() clientset.Interface {
	return c.remoteClient
}

func (c *Option) KosmosClient() versioned.Interface {
	return c.kosmosClient
}

func (c *Option) UpdateVirtualCluster(vc *v1alpha1.VirtualCluster) {
	c.virtualCluster = vc
}

type TaskFunc func(*Option) error

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
func RunTask(tasks []TaskFunc, r *Option) error {
	total := len(tasks)
	for index, task := range tasks {
		klog.Infof("###################### Running task (%d/%d): [%s] \n", index+1, total, getFunctionName(task))
		err := task(r)
		if err != nil {
			return err
		}
	}
	return nil
}

package membercurd

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/util/apiclient"
	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned/typed/clusterlink/v1alpha1"
)

type CommandMemberOption struct {
	ImageRegistry    string
	NameSpace        string
	KubeClientSet    v1alpha1.ClusterInterface
	Context          string
	ClusterName      string
	KubeConfig       string
	MemberKubeConfig string
	RestConfig       *rest.Config
	CNIPlugin        string
	NetworkType      string
}

type ClusterInfo struct {
	Name             string
	CNI              string
	ImageRepository  string
	NetworkType      string
	APIServerAddress string
}

var infoList []ClusterInfo

// InitKubeClient Initialize a kubernetes client
func (m *CommandMemberOption) InitKubeClient() error {

	kubeconfigPath := apiclient.KubeConfigPath(m.KubeConfig)
	if !apiclient.Exists(kubeconfigPath) {
		klog.Fatal("kubeconfig no found")
		return fmt.Errorf("kubeconfig no found")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}
	m.RestConfig = config
	clientSet, err := versioned.NewForConfig(config)
	if err != nil {
		return err
	}
	clusterClient := clientSet.ClusterlinkV1alpha1().Clusters()
	m.KubeClientSet = clusterClient

	return nil
}

func (m *CommandMemberOption) ShowKubeCluster() error {
	clusterList, err := m.KubeClientSet.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("List Cluster error : %v", err)
		return err
	}
	if clusterList == nil || len(clusterList.Items) == 0 {
		fmt.Println("No resources found")
	}
	var wg sync.WaitGroup
	mux := sync.Mutex{}
	wg.Add(len(clusterList.Items))
	for _, cluster := range clusterList.Items {
		go func() {
			err := m.GetInfoFromCluster(&wg, &mux, cluster)
			if err != nil {
				klog.Errorf("GetInfoFromCluster(%s) err: %v", cluster.Name, err)
			}
		}()
	}
	wg.Wait()
	if len(infoList) != 0 {
		_, err = fmt.Printf("%-20s\t|%-10s\t|%-10s\t|%-30s\t|%-30s|\n", "Name",
			"CNIPlugin", "NetworkType", "APIServerAddress", "ImageRepository")
		if err != nil {
			klog.Errorf("print cluster info header error : %v", err)
		}
		for _, info := range infoList {
			_, err = fmt.Printf("%-20s\t|%-10s\t|%-10s\t|%-30s\t|%-30s|\n", info.Name, info.CNI, info.NetworkType,
				info.APIServerAddress, info.ImageRepository)
			if err != nil {
				klog.Errorf("print cluster info body error : %v", err)
			}
		}
	}
	return nil
}

func (m *CommandMemberOption) GetInfoFromCluster(wg *sync.WaitGroup, mux *sync.Mutex,
	cluster clusterlinkv1alpha1.Cluster) error {

	/*
		defer wg.Done()
		kubeconfigDecode := cluster.Spec.Kubeconfig
		clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfigDecode)
		if err != nil {
			return err
		}
		config, err := clientConfig.ClientConfig()
		if err != nil {
			return err
		}

		mux.Lock()
		infoList = append(infoList, ClusterInfo{
			Name:             cluster.Name,
			ImageRepository:  cluster.Spec.ImageRepository,
			CNI:              cluster.Spec.CNI,
			NetworkType:      string(cluster.Spec.NetworkType),
			APIServerAddress: config.Host,
		})
		mux.Unlock()
	*/
	return nil
}

func (m *CommandMemberOption) DelKubeCluster() error {

	_, err := m.KubeClientSet.Get(context.TODO(), m.ClusterName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("the cluster %s does not exit", m.ClusterName)
		return err
	}
	patchData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": []interface{}{},
		},
	}
	mergeJson, err := json.Marshal(patchData)
	if err != nil {
		klog.Errorf("Json marshal error : %v", err)
		return err
	}
	_, err = m.KubeClientSet.Patch(context.TODO(), m.ClusterName, types.MergePatchType,
		mergeJson, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("Patch Cluster error : %v", err)
		return err
	}

	err = m.KubeClientSet.Delete(context.TODO(), m.ClusterName, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Remove Cluster error : %v", err)
		return err
	}
	fmt.Printf("Cluster %s unjoin succeccfully!\n", m.ClusterName)

	return nil
}

func (m *CommandMemberOption) AddKubeCluster() error {

	if m.NetworkType != "gateway" && m.NetworkType != "p2p" {
		return fmt.Errorf("strange NetworkType %s,can't recognize", m.NetworkType)
	}
	/*
		var nt clusterlinkv1alpha1.NetworkType
		if m.NetworkType == "gateway" {
			nt = clusterlinkv1alpha1.NetWorkTypeGateWay
		} else {
			nt = clusterlinkv1alpha1.NetworkTypeP2P
		}
	*/
	if m.NameSpace == "" {
		m.NameSpace = "clusterlink-system"
	}

	kubeconfigPath := apiclient.KubeConfigPath(m.MemberKubeConfig)
	if !apiclient.Exists(kubeconfigPath) {
		klog.Fatal("Member kubeconfig no found")
		return fmt.Errorf("merber kubeconfig no found")
	}
	/*
		content, err := os.ReadFile(kubeconfigPath)
		if err != nil {
			return err
		}
	*/

	/*
		cluster := &clusterlinkv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: m.ClusterName,
			},
			Spec: clusterlinkv1alpha1.ClusterSpec{
				NetworkType:     nt,
				CNI:             m.CNIPlugin,
				ImageRepository: m.ImageRegistry,
				//Kubeconfig:      []byte(content),
				Namespace: m.NameSpace,
			},
		}
	*/
	/*
		_, err = m.KubeClientSet.Create(context.TODO(), cluster, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("Create Cluster error : %v", err)
			return err
		}

	*/
	fmt.Printf("Cluster %s join succeccfully!\n", m.ClusterName)

	return nil
}

func (m *CommandMemberOption) Validate(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("only support the clustername as argument")
	}
	if len(args) == 0 {
		return fmt.Errorf("cluster name should be provided")
	}
	m.ClusterName = args[0]
	k8sApiServerIP := m.RestConfig.Host
	pattern := regexp.MustCompile(`https?://\[?([a-f0-9.:]+)\]?:.*`)
	res := pattern.FindStringSubmatch(k8sApiServerIP)
	if len(res) != 2 {
		return fmt.Errorf("resolve api-server ipaddress form kubeconfig failed")
	}
	// TODO: Validate cluster is already under control
	//k8sApiServerIP = res[1]

	return nil
}

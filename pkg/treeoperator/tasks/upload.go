package tasks

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/treeoperator/cert"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/util"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/workflow"
)

const (
	VirtualClusterLabelKeyName = "app.kubernetes.io/managed-by"
	VirtualClusterController   = "virtual-cluster-controller"
	EtcdCaCertAndKeyName       = "etcd-ca"
	EtcdServerCertAndKeyName   = "etcd-server"
	EtcdClientCertAndKeyName   = "etcd-client"
	CaCertAndKeyName           = "ca"
	ClusterName                = "virtualCluster-apiserver"
	UserName                   = "virtualCluster-admin"
)

var (
	VirtualClusterControllerLabel = labels.Set{VirtualClusterLabelKeyName: VirtualClusterController}
)

func NewUploadCertsTask() workflow.Task {
	return workflow.Task{
		Name:        "Upload-Certs",
		Run:         runUploadCerts,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "Upload-VirtualClusterCert",
				Run:  runUploadVirtualClusterCert,
			},
			{
				Name: "Upload-EtcdCert",
				Run:  runUploadEtcdCert,
			},
		},
	}
}

func NewUploadKubeconfigTask() workflow.Task {
	return workflow.Task{
		Name:        "upload-config",
		RunSubTasks: true,
		Run:         runUploadKubeconfig,
		Tasks: []workflow.Task{
			{
				Name: "UploadAdminKubeconfig",
				Run:  runUploadAdminKubeconfig,
			},
		},
	}
}

func runUploadCerts(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-certs task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[upload-certs] Running upload-certs task", "virtual cluster", klog.KObj(data))

	if len(data.CertList()) == 0 {
		return errors.New("there is no certs in store, please reload certs to store")
	}
	return nil
}

func runUploadKubeconfig(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-config task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[upload-config] Running task", "virtual cluster", klog.KObj(data))
	return nil
}

func runUploadVirtualClusterCert(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-VirtualClusterCert task invoked with an invalid data struct")
	}

	certList := data.CertList()
	certsData := make(map[string][]byte, len(certList))
	for _, c := range certList {
		certsData[c.KeyName()] = c.KeyData()
		certsData[c.CertName()] = c.CertData()
	}

	err := createOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", data.GetName(), "cert"),
			Namespace: data.GetNamespace(),
			Labels:    labels.Set{VirtualClusterLabelKeyName: VirtualClusterController},
		},
		Data: certsData,
	})
	if err != nil {
		return fmt.Errorf("failed to upload virtual cluster cert to secret, err: %w", err)
	}

	klog.V(2).InfoS("[upload-VirtualClusterCert] Successfully uploaded virtual cluster certs to secret", "virtual cluster", klog.KObj(data))
	return nil
}

func runUploadEtcdCert(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-etcdCert task invoked with an invalid data struct")
	}

	ca := data.GetCert(EtcdCaCertAndKeyName)
	server := data.GetCert(EtcdServerCertAndKeyName)
	client := data.GetCert(EtcdClientCertAndKeyName)

	err := createOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: data.GetNamespace(),
			Name:      fmt.Sprintf("%s-%s", data.GetName(), "etcd-cert"),
			Labels:    labels.Set{VirtualClusterLabelKeyName: VirtualClusterController},
		},

		Data: map[string][]byte{
			ca.CertName():     ca.CertData(),
			ca.KeyName():      ca.KeyData(),
			server.CertName(): server.CertData(),
			server.KeyName():  server.KeyData(),
			client.CertName(): client.CertData(),
			client.KeyName():  client.KeyData(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upload etcd certs to secret, err: %w", err)
	}

	klog.V(2).InfoS("[upload-etcdCert] Successfully uploaded etcd certs to secret", "virtual cluster", klog.KObj(data))
	return nil
}

func createOrUpdateSecret(client clientset.Interface, secret *corev1.Secret) error {
	_, err := client.CoreV1().Secrets(secret.GetNamespace()).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.CoreV1().Secrets(secret.GetNamespace()).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated secret", "secret", secret.GetName())
	return nil
}

func runUploadAdminKubeconfig(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("UploadAdminKubeconfig task invoked with an invalid data struct")
	}

	var endpoint string

	service, err := data.RemoteClient().CoreV1().Services(data.GetNamespace()).Get(context.TODO(), fmt.Sprintf("%s-%s", data.GetName(), "apiserver"), metav1.GetOptions{})
	if err != nil {
		return err
	}
	nodePort := getNodePortFromAPIServerService(service)
	endpoint = fmt.Sprintf("https://%s:%d", data.ControlplaneAddress(), nodePort)

	kubeconfig, err := buildKubeConfigFromSpec(data, endpoint)
	if err != nil {
		return err
	}

	configBytes, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return err
	}

	err = createOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: data.GetNamespace(),
			Name:      fmt.Sprintf("%s-%s", data.GetName(), "admin-config"),
			Labels:    VirtualClusterControllerLabel,
		},
		Data: map[string][]byte{"kubeconfig": configBytes},
	})
	if err != nil {
		return fmt.Errorf("failed to create secret of kubeconfig, err: %w", err)
	}

	// store rest config to RunData.
	config, err := clientcmd.RESTConfigFromKubeConfig(configBytes)
	if err != nil {
		return err
	}
	data.SetControlplaneConfig(config)

	klog.V(2).InfoS("[UploadAdminKubeconfig] Successfully created secret of virtual cluster apiserver kubeconfig", "virtual cluster", klog.KObj(data))
	return nil
}

func getNodePortFromAPIServerService(service *corev1.Service) int32 {
	var nodePort int32
	if service.Spec.Type == corev1.ServiceTypeNodePort {
		for _, port := range service.Spec.Ports {
			if port.Name != "client" {
				continue
			}
			nodePort = port.NodePort
		}
	}

	return nodePort
}

func buildKubeConfigFromSpec(data InitData, serverURL string) (*clientcmdapi.Config, error) {
	ca := data.GetCert(CaCertAndKeyName)
	if ca == nil {
		return nil, errors.New("unable build virtual cluster admin kubeconfig, CA cert is empty")
	}

	cc := cert.VirtualClusterCertClient()

	if err := mutateCertConfig(data, cc); err != nil {
		return nil, fmt.Errorf("error when mutate cert altNames for %s, err: %w", cc.Name, err)
	}
	client, err := cert.CreateCertAndKeyFilesWithCA(cc, ca.CertData(), ca.KeyData())
	if err != nil {
		return nil, fmt.Errorf("failed to generate virtual cluster apiserver client certificate for kubeconfig, err: %w", err)
	}

	return util.CreateWithCerts(
		serverURL,
		ClusterName,
		UserName,
		ca.CertData(),
		client.KeyData(),
		client.CertData(),
	), nil
}

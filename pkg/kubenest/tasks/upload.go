package tasks

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util/cert"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

var (
	VirtualClusterControllerLabel = labels.Set{constants.VirtualClusterLabelKeyName: constants.VirtualClusterController}
)

type PortInfo struct {
	NodePort      int32
	ClusterIPPort int32
}

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
			Labels:    VirtualClusterControllerLabel,
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

	ca := data.GetCert(constants.EtcdCaCertAndKeyName)
	server := data.GetCert(constants.EtcdServerCertAndKeyName)
	client := data.GetCert(constants.EtcdClientCertAndKeyName)

	err := createOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: data.GetNamespace(),
			Name:      fmt.Sprintf("%s-%s", data.GetName(), "etcd-cert"),
			Labels:    VirtualClusterControllerLabel,
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

	var controlplaneIpEndpoint, clusterIPEndpoint string
	service, err := data.RemoteClient().CoreV1().Services(data.GetNamespace()).Get(context.TODO(), fmt.Sprintf("%s-%s", data.GetName(), "apiserver"), metav1.GetOptions{})
	if err != nil {
		return err
	}
	portInfo := getPortInfoFromAPIServerService(service)
	// controlplane address + nodePort
	controlplaneIpEndpoint = fmt.Sprintf("https://%s:%d", data.ControlplaneAddress(), portInfo.NodePort)
	controlplaneIpKubeconfig, err := buildKubeConfigFromSpec(data, controlplaneIpEndpoint)
	if err != nil {
		return err
	}

	//clusterIP address + clusterIPPort
	clusterIPEndpoint = fmt.Sprintf("https://%s:%d", service.Spec.ClusterIP, portInfo.ClusterIPPort)
	clusterIPKubeconfig, err := buildKubeConfigFromSpec(data, clusterIPEndpoint)
	if err != nil {
		return err
	}

	controlplaneIpConfigBytes, err := clientcmd.Write(*controlplaneIpKubeconfig)
	if err != nil {
		return err
	}

	clusterIPConfigBytes, err := clientcmd.Write(*clusterIPKubeconfig)
	if err != nil {
		return err
	}

	err = createOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: data.GetNamespace(),
			Name:      fmt.Sprintf("%s-%s", data.GetName(), "admin-config"),
			Labels:    VirtualClusterControllerLabel,
		},
		Data: map[string][]byte{"kubeconfig": controlplaneIpConfigBytes},
	})
	if err != nil {
		return fmt.Errorf("failed to create secret of kubeconfig, err: %w", err)
	}

	err = createOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: data.GetNamespace(),
			Name:      fmt.Sprintf("%s-%s", data.GetName(), "admin-config-clusterip"),
			Labels:    VirtualClusterControllerLabel,
		},
		Data: map[string][]byte{"kubeconfig": clusterIPConfigBytes},
	})
	if err != nil {
		return fmt.Errorf("failed to create secret of kubeconfig-clusterip, err: %w", err)
	}

	klog.V(2).InfoS("[UploadAdminKubeconfig] Successfully created secrets of virtual cluster apiserver kubeconfig", "virtual cluster", klog.KObj(data))
	return nil
}

func getPortInfoFromAPIServerService(service *corev1.Service) PortInfo {
	var portInfo PortInfo
	//var nodePort int32
	if service.Spec.Type == corev1.ServiceTypeNodePort {
		for _, port := range service.Spec.Ports {
			if port.Name != constants.APIServerSVCPortName {
				continue
			}
			portInfo.NodePort = port.NodePort
			portInfo.ClusterIPPort = port.Port
		}
	}

	return portInfo
}

func buildKubeConfigFromSpec(data InitData, serverURL string) (*clientcmdapi.Config, error) {
	ca := data.GetCert(constants.CaCertAndKeyName)
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
		constants.ClusterName,
		constants.UserName,
		ca.CertData(),
		client.KeyData(),
		client.CertData(),
	), nil
}

func UninstallCertsAndKubeconfigTask() workflow.Task {
	return workflow.Task{
		Name:        "Uninstall-Certs",
		Run:         runUninstallCerts,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "Uninstall-Certs",
				Run:  deleteSecrets,
			},
		},
	}
}

func runUninstallCerts(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Uninstall-Certs task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[uninstall-Certs] Running task", "virtual cluster", klog.KObj(data))
	return nil
}

func deleteSecrets(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-VirtualClusterCert task invoked with an invalid data struct")
	}

	secrets := []string{
		fmt.Sprintf("%s-%s", data.GetName(), "cert"),
		fmt.Sprintf("%s-%s", data.GetName(), "etcd-cert"),
		fmt.Sprintf("%s-%s", data.GetName(), "admin-config"),
		fmt.Sprintf("%s-%s", data.GetName(), "admin-config-clusterip"),
	}
	for _, secret := range secrets {
		err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Delete(context.TODO(), secret, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.V(2).Infof("Secret %s/%s not found, skip delete", secret, data.GetNamespace())
				continue
			}
			return errors.Wrapf(err, "Failed to delete secret %s/%s", secret, data.GetNamespace())
		}
	}
	klog.V(2).Infof("Successfully uninstalled virtual cluster %s secrets", data.GetName())
	return nil
}

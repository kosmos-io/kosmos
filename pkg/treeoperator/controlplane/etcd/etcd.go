package etcd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/treeoperator/constants"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	Replicas             = 1
	ImageDefaultVersion  = "3.5.9-0"
	EtcdDataVolumeName   = "etcd-data"
	EtcdListenClientPort = 2379
	EtcdListenPeerPort   = 2380
)

var errAllocated = errors.New("provided port is already allocated")

func EnsureKarmadaEtcd(client clientset.Interface, name, namespace string) error {
	if err := installEtcd(client, name, namespace); err != nil {
		return err
	}
	return createEtcdService(client, name, namespace)
}

func installEtcd(client clientset.Interface, name, namespace string) error {
	// if the number of etcd is greater than one, we need to concatenate the peerURL for each member cluster.
	// memberName is podName generated by etcd statefulset: ${statefulsetName}-index
	// memberPeerURL uses the etcd peer headless service name: ${podName}.${serviceName}.${namespace}.svc.cluster.local:2380
	initialClusters := make([]string, Replicas)
	for index := range initialClusters {
		memberName := fmt.Sprintf("%s-%d", fmt.Sprintf("%s-%s", name, "etcd"), index)

		// build etcd member cluster peer url
		memberPeerURL := fmt.Sprintf("http://%s.%s.%s.svc.cluster.local:%v",
			memberName,
			fmt.Sprintf("%s-%s", name, "etcd"),
			namespace,
			EtcdListenPeerPort,
		)

		initialClusters[index] = fmt.Sprintf("%s=%s", memberName, memberPeerURL)
	}
	imageRepository := os.Getenv(constants.DefauleImageRepositoryEnv)
	if len(imageRepository) == 0 {
		imageRepository = utils.DefaultImageRepository
	}
	etcdStatefulSetBytes, err := util.ParseTemplate(EtcdStatefulSet, struct {
		StatefulSetName, Namespace, ImageRepository, Image, EtcdClientService, Version string
		CertsSecretName, EtcdPeerServiceName                                           string
		InitialCluster, EtcdDataVolumeName, EtcdCipherSuites                           string
		Replicas, EtcdListenClientPort, EtcdListenPeerPort                             int32
	}{
		StatefulSetName:      fmt.Sprintf("%s-%s", name, "etcd"),
		Namespace:            namespace,
		ImageRepository:      imageRepository,
		Version:              ImageDefaultVersion,
		EtcdClientService:    fmt.Sprintf("%s-%s", name, "etcd-client"),
		CertsSecretName:      fmt.Sprintf("%s-%s", name, "etcd-cert"),
		EtcdPeerServiceName:  fmt.Sprintf("%s-%s", name, "etcd"),
		EtcdDataVolumeName:   EtcdDataVolumeName,
		InitialCluster:       strings.Join(initialClusters, ","),
		EtcdCipherSuites:     genEtcdCipherSuites(),
		Replicas:             Replicas,
		EtcdListenClientPort: EtcdListenClientPort,
		EtcdListenPeerPort:   EtcdListenPeerPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Etcd statefuelset template: %w", err)
	}

	etcdStatefulSet := &appsv1.StatefulSet{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), etcdStatefulSetBytes, etcdStatefulSet); err != nil {
		return fmt.Errorf("error when decoding Etcd StatefulSet: %w", err)
	}

	if err := createOrUpdateStatefulSet(client, etcdStatefulSet); err != nil {
		return fmt.Errorf("error when creating Etcd statefulset, err: %w", err)
	}

	return nil
}

func createOrUpdateStatefulSet(client clientset.Interface, statefulSet *appsv1.StatefulSet) error {
	_, err := client.AppsV1().StatefulSets(statefulSet.GetNamespace()).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		older, err := client.AppsV1().StatefulSets(statefulSet.GetNamespace()).Get(context.TODO(), statefulSet.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		statefulSet.ResourceVersion = older.ResourceVersion
		_, err = client.AppsV1().StatefulSets(statefulSet.GetNamespace()).Update(context.TODO(), statefulSet, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated statefulset", "statefulset", statefulSet.GetName)
	return nil
}

func createEtcdService(client clientset.Interface, name, namespace string) error {
	etcdServicePeerBytes, err := util.ParseTemplate(EtcdPeerService, struct {
		ServiceName, Namespace                   string
		EtcdListenClientPort, EtcdListenPeerPort int32
	}{
		ServiceName:          fmt.Sprintf("%s-%s", name, "etcd"),
		Namespace:            namespace,
		EtcdListenClientPort: EtcdListenClientPort,
		EtcdListenPeerPort:   EtcdListenPeerPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Etcd client serive template: %w", err)
	}

	etcdPeerService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), etcdServicePeerBytes, etcdPeerService); err != nil {
		return fmt.Errorf("error when decoding Etcd client service: %w", err)
	}

	if err := createOrUpdateService(client, etcdPeerService); err != nil {
		return fmt.Errorf("error when creating etcd client service, err: %w", err)
	}

	etcdClientServiceBytes, err := util.ParseTemplate(EtcdClientService, struct {
		ServiceName, Namespace string
		EtcdListenClientPort   int32
	}{
		ServiceName:          fmt.Sprintf("%s-%s", name, "etcd-client"),
		Namespace:            namespace,
		EtcdListenClientPort: EtcdListenClientPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Etcd client serive template: %w", err)
	}

	etcdClientService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), etcdClientServiceBytes, etcdClientService); err != nil {
		return fmt.Errorf("err when decoding Etcd client service: %w", err)
	}

	if err := createOrUpdateService(client, etcdClientService); err != nil {
		return fmt.Errorf("err when creating etcd client service, err: %w", err)
	}

	return nil
}

func createOrUpdateService(client clientset.Interface, service *corev1.Service) error {
	_, err := client.CoreV1().Services(service.GetNamespace()).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			// Ignore if the Service is invalid with this error message:
			// Service "apiserver" is invalid: provided Port is already allocated.
			if apierrors.IsInvalid(err) && strings.Contains(err.Error(), errAllocated.Error()) {
				klog.V(2).ErrorS(err, "failed to create or update service", "service", klog.KObj(service))
				return nil
			}
			return fmt.Errorf("unable to create Service: %v", err)
		}

		older, err := client.CoreV1().Services(service.GetNamespace()).Get(context.TODO(), service.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		service.ResourceVersion = older.ResourceVersion
		if _, err := client.CoreV1().Services(service.GetNamespace()).Update(context.TODO(), service, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update Service: %v", err)
		}
	}

	klog.V(5).InfoS("Successfully created or updated service", "service", service.GetName())
	return nil
}

func genEtcdCipherSuites() string {
	return strings.Join(flag.PreferredTLSCipherNames(), ",")
}

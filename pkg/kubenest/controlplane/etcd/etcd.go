package etcd

import (
	"context"
	"errors"
	"fmt"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/etcd"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

var errAllocated = errors.New("provided port is already allocated")

func EnsureVirtualClusterEtcd(client clientset.Interface, name, namespace string) error {
	if err := installEtcd(client, name, namespace); err != nil {
		return err
	}
	return nil
}

func installEtcd(client clientset.Interface, name, namespace string) error {
	imageRepository, imageVersion := util.GetImageMessage()
	initialClusters := make([]string, constants.EtcdReplicas)
	for index := range initialClusters {
		memberName := fmt.Sprintf("%s-%d", fmt.Sprintf("%s-%s", name, "etcd"), index)
		// build etcd member cluster peer url
		memberPeerURL := fmt.Sprintf("http://%s.%s.%s.svc.cluster.local:%v",
			memberName,
			fmt.Sprintf("%s-%s", name, "etcd"),
			namespace,
			constants.EtcdListenPeerPort,
		)

		initialClusters[index] = fmt.Sprintf("%s=%s", memberName, memberPeerURL)
	}

	etcdStatefulSetBytes, err := util.ParseTemplate(etcd.EtcdStatefulSet, struct {
		StatefulSetName, Namespace, ImageRepository, Image, EtcdClientService, Version string
		CertsSecretName, EtcdPeerServiceName                                           string
		InitialCluster, EtcdDataVolumeName, EtcdCipherSuites                           string
		Replicas, EtcdListenClientPort, EtcdListenPeerPort                             int32
	}{
		StatefulSetName:      fmt.Sprintf("%s-%s", name, "etcd"),
		Namespace:            namespace,
		ImageRepository:      imageRepository,
		Version:              imageVersion,
		EtcdClientService:    fmt.Sprintf("%s-%s", name, "etcd-client"),
		CertsSecretName:      fmt.Sprintf("%s-%s", name, "etcd-cert"),
		EtcdPeerServiceName:  fmt.Sprintf("%s-%s", name, "etcd"),
		EtcdDataVolumeName:   constants.EtcdDataVolumeName,
		InitialCluster:       strings.Join(initialClusters, ","),
		EtcdCipherSuites:     strings.Join(flag.PreferredTLSCipherNames(), ","),
		Replicas:             constants.EtcdReplicas,
		EtcdListenClientPort: constants.EtcdListenClientPort,
		EtcdListenPeerPort:   constants.EtcdListenPeerPort,
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

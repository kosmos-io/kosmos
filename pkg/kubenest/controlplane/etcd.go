package controlplane

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/component-base/cli/flag"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/etcd"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureVirtualClusterEtcd(client clientset.Interface, name, namespace string) error {
	if err := installEtcd(client, name, namespace); err != nil {
		return err
	}
	return nil
}

func DeleteVirtualClusterEtcd(client clientset.Interface, name, namespace string) error {
	sts := fmt.Sprintf("%s-%s", name, "etcd")
	if err := util.DeleteStatefulSet(client, sts, namespace); err != nil {
		return errors.Wrapf(err, "Failed to delete statefulset %s/%s", sts, namespace)
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
	if err := yaml.Unmarshal([]byte(etcdStatefulSetBytes), etcdStatefulSet); err != nil {
		return fmt.Errorf("error when decoding Etcd StatefulSet: %w", err)
	}

	if err := util.CreateOrUpdateStatefulSet(client, etcdStatefulSet); err != nil {
		return fmt.Errorf("error when creating Etcd statefulset, err: %w", err)
	}

	return nil
}

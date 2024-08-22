package controlplane

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/component-base/cli/flag"
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/etcd"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureVirtualClusterEtcd(client clientset.Interface, name, namespace string, kubeNestConfiguration *v1alpha1.KubeNestConfiguration, vc *v1alpha1.VirtualCluster) error {
	if err := installEtcd(client, name, namespace, kubeNestConfiguration, vc); err != nil {
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

func installEtcd(client clientset.Interface, name, namespace string, kubeNestConfiguration *v1alpha1.KubeNestConfiguration, vc *v1alpha1.VirtualCluster) error {
	imageRepository, imageVersion := util.GetImageMessage()

	nodeCount := getNodeCountFromPromotePolicy(vc)
	resourceQuantity, err := resource.ParseQuantity(kubeNestConfiguration.KubeInKubeConfig.ETCDUnitSize)
	if err != nil {
		klog.Errorf("Failed to parse quantity %s: %v", kubeNestConfiguration.KubeInKubeConfig.ETCDUnitSize, err)
		return err
	}
	resourceQuantity.Set(resourceQuantity.Value() * int64(nodeCount))

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

	vclabel := util.GetVirtualControllerLabel()

	IPV6FirstFlag, err := util.IPV6First(constants.ApiServerServiceSubnet)
	if err != nil {
		return err
	}

	etcdStatefulSetBytes, err := util.ParseTemplate(etcd.EtcdStatefulSet, struct {
		StatefulSetName, Namespace, ImageRepository, Image, EtcdClientService, Version, VirtualControllerLabel string
		CertsSecretName, EtcdPeerServiceName                                                                   string
		InitialCluster, EtcdDataVolumeName, EtcdCipherSuites                                                   string
		Replicas, EtcdListenClientPort, EtcdListenPeerPort                                                     int32
		ETCDStorageClass, ETCDStorageSize                                                                      string
		IPV6First                                                                                              bool
	}{
		StatefulSetName:        fmt.Sprintf("%s-%s", name, "etcd"),
		Namespace:              namespace,
		ImageRepository:        imageRepository,
		Version:                imageVersion,
		VirtualControllerLabel: vclabel,
		EtcdClientService:      fmt.Sprintf("%s-%s", name, "etcd-client"),
		CertsSecretName:        fmt.Sprintf("%s-%s", name, "etcd-cert"),
		EtcdPeerServiceName:    fmt.Sprintf("%s-%s", name, "etcd"),
		EtcdDataVolumeName:     constants.EtcdDataVolumeName,
		InitialCluster:         strings.Join(initialClusters, ","),
		EtcdCipherSuites:       strings.Join(flag.PreferredTLSCipherNames(), ","),
		Replicas:               constants.EtcdReplicas,
		EtcdListenClientPort:   constants.EtcdListenClientPort,
		EtcdListenPeerPort:     constants.EtcdListenPeerPort,
		ETCDStorageClass:       kubeNestConfiguration.KubeInKubeConfig.ETCDStorageClass,
		ETCDStorageSize:        resourceQuantity.String(),
		IPV6First:              IPV6FirstFlag,
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

func getNodeCountFromPromotePolicy(vc *v1alpha1.VirtualCluster) int32 {
	var nodeCount int32
	for _, policy := range vc.Spec.PromotePolicies {
		nodeCount = nodeCount + policy.NodeCount
	}
	return nodeCount
}

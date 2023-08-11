package ctlmaster

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/util/apiclient"
	"github.com/kosmos.io/clusterlink/pkg/operator/util"
	"github.com/kosmos.io/clusterlink/pkg/version"
)

var (
	clusterlinkRelease string

	// DefaultCrdURL crds resource
	DefaultCrdURL string
	// DefaultInitImage etcd init container image
	DefaultInitImage                         string
	DefaultClusterlinkOperatorImage          string
	DefaultClusterlinkControllerManagerImage string
	DefaultClusterlinkOperatorReplicas       int32
)

func init() {
	releaseVer := version.GetReleaseVersion()
	clusterlinkRelease = releaseVer.PatchRelease()

	DefaultCrdURL = fmt.Sprintf("https://github.com/clusterlink-io/clusterlink/releases/download/%s/crds.tar.gz", releaseVer.FirstMinorRelease())
	DefaultInitImage = "docker.io/alpine:3.15.1"
	DefaultClusterlinkOperatorImage = "ghcr.io/kosmos-io/clusterlink/clusterlink-operator:0.1.0" // fmt.Sprintf("docker.io/clusterlink/clusterlink-operator:%s", releaseVer.PatchRelease())
	DefaultClusterlinkOperatorReplicas = 1
	DefaultClusterlinkControllerManagerImage = "ghcr.io/kosmos-io/clusterlink/clusterlink-controller-manager:0.1.0" //fmt.Sprintf("docker.io/clusterlink/clusterlink-controller-manager:%s", releaseVer.PatchRelease())
}

// CommandInitOption holds all flags options for init.
type CommandInitOption struct {
	ImageRegistry                 string
	ClusterlinkOperatorImage      string
	ClusterlinkOperatorReplicas   int32
	ClusterlinkControllerImage    string
	ClusterlinkControllerReplicas int32

	Namespace              string
	KubeConfig             string
	CRDs                   string
	ExternalDNS            string
	PullSecrets            []string
	CertValidity           time.Duration
	KubeClientSet          kubernetes.Interface
	ExtensionKubeClientSet apiextensionsclientset.Interface
	CertAndKeyFileData     map[string][]byte
}

// InitKubeClient Initialize a kubernetes client
func (i *CommandInitOption) InitKubeClient() error {

	_, normalClinet, extendClient, err := apiclient.CreateKubeClient(i.KubeConfig)
	if err != nil {
		return err
	}
	i.KubeClientSet = normalClinet
	i.ExtensionKubeClientSet = extendClient

	return nil
}

// MapToString  labels to string
func MapToString(labels map[string]string) string {
	v := new(bytes.Buffer)
	for key, value := range labels {
		_, err := fmt.Fprintf(v, "%s=%s,", key, value)
		if err != nil {
			klog.Errorf("map to string error: %s", err)
		}
	}
	return strings.TrimRight(v.String(), ",")
}

// RunInit Deploy clusterlink in kubernetes
func (i *CommandInitOption) RunInit(parentCommand string) error {

	// Create ns
	klog.Infof("Create namespace %s", i.Namespace)
	if err := util.CreateOrUpdateNamespace(i.KubeClientSet, util.NewNamespace(i.Namespace)); err != nil {
		return fmt.Errorf("create namespace %s failed: %v", i.Namespace, err)
	}
	// install RBAC
	if err := i.initClusterlinkRBAC(); err != nil {
		return err
	}

	// install clusterlink crd
	if err := i.initClusterlinkCRDs(); err != nil {
		return err
	}
	// create clusterlink configmap
	if err := i.initClusterlinkConfigmap(); err != nil {
		return err
	}
	// install clusterlink-operator and clusterlink-controller-manager
	if err := i.initClusterlinkDeployment(); err != nil {
		return err
	}

	return nil
}

// get registry
func (i *CommandInitOption) kubeRegistry() string {

	if i.ImageRegistry != "" {
		return i.ImageRegistry
	}
	return "docker.io/clusterlink"
}

func (i *CommandInitOption) getImageByAppName(appName string) string {

	if i.ImageRegistry != "" {
		return fmt.Sprintf("%s/%s:%s", i.kubeRegistry(), appName, clusterlinkRelease) // i.ImageRegistry + "/clusterlink-operator:" + clusterlinkRelease
	}

	if appName == "clusterlink-operator" {
		if i.ImageRegistry == "" && i.ClusterlinkOperatorImage == "" {
			return DefaultClusterlinkOperatorImage
		} else if i.ClusterlinkOperatorImage != "" {
			return i.ClusterlinkOperatorImage
		}
	} else if appName == "clusterlink-controller-manager" {
		if i.ImageRegistry == "" && i.ClusterlinkControllerImage == "" {
			return DefaultClusterlinkControllerManagerImage
		} else if i.ClusterlinkControllerImage != "" {
			return i.ClusterlinkControllerImage
		}
	}

	return ""
}

// GetImagePullSecrets get image pull secret
func (i *CommandInitOption) GetImagePullSecrets() []corev1.LocalObjectReference {
	var imagePullSecrets []corev1.LocalObjectReference
	for _, val := range i.PullSecrets {
		secret := corev1.LocalObjectReference{
			Name: val,
		}
		imagePullSecrets = append(imagePullSecrets, secret)
	}
	return imagePullSecrets
}

package k8sadapter

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils/manager"
)

const RooTCAConfigMapName = "kube-root-ca.crt"
const SATokenPrefix = "kube-api-access"
const MasterRooTCAName = "master-root-ca.crt"

// ClientConfig defines the configuration of a lower cluster
type ClientConfig struct {
	// allowed qps of the kube client
	KubeClientQPS int
	// allowed burst of the kube client
	KubeClientBurst int
	// config path of the kube client
	ClientKubeConfig []byte
}

type clientCache struct {
	podLister    v1.PodLister
	nsLister     v1.NamespaceLister
	cmLister     v1.ConfigMapLister
	secretLister v1.SecretLister
	nodeLister   v1.NodeLister
}

type PodAdapterConfig struct {
	ConfigPath        string
	NodeName          string
	OperatingSystem   string
	InternalIP        string
	DaemonPort        int32
	KubeClusterDomain string
	ResourceManager   *manager.ResourceManager
}

type PodAdapter struct {
	master               kubernetes.Interface
	client               kubernetes.Interface
	ignoreLabels         []string
	clientCache          clientCache
	rm                   *manager.ResourceManager
	updatedPod           chan *corev1.Pod
	enableServiceAccount bool
	stopCh               <-chan struct{}
}

func NewPodAdapter(cfg PodAdapterConfig, ignoreLabelsStr string, cc *ClientConfig, enableServiceAccount bool) (*PodAdapter, error) {
	ignoreLabels := strings.Split(ignoreLabelsStr, ",")
	if len(cc.ClientKubeConfig) == 0 {
		panic("client kubeconfig path can not be empty")
	}
	// client config
	// var clientConfig *rest.Config
	client, err := utils.NewClientFromByte(cc.ClientKubeConfig, func(config *rest.Config) {
		config.QPS = float32(cc.KubeClientQPS)
		config.Burst = cc.KubeClientBurst
		// Set config for clientConfig
		// clientConfig = config
	})
	if err != nil {
		return nil, fmt.Errorf("could not build clientset for cluster: %v", err)
	}

	// master config, maybe a real node or a pod
	master, err := utils.NewClient(cfg.ConfigPath, func(config *rest.Config) {
		// config.QPS = float32(opts.KubeAPIQPS)
		// config.Burst = int(opts.KubeAPIBurst)
	})
	if err != nil {
		return nil, fmt.Errorf("could not build clientset for cluster: %v", err)
	}

	informer := kubeinformers.NewSharedInformerFactory(client, 0)
	podInformer := informer.Core().V1().Pods()
	nsInformer := informer.Core().V1().Namespaces()
	nodeInformer := informer.Core().V1().Nodes()
	cmInformer := informer.Core().V1().ConfigMaps()
	secretInformer := informer.Core().V1().Secrets()

	ctx := context.TODO()

	return &PodAdapter{
		master:               master,
		client:               client,
		ignoreLabels:         ignoreLabels,
		enableServiceAccount: enableServiceAccount,
		clientCache: clientCache{
			podLister:    podInformer.Lister(),
			nsLister:     nsInformer.Lister(),
			cmLister:     cmInformer.Lister(),
			secretLister: secretInformer.Lister(),
			nodeLister:   nodeInformer.Lister(),
		},
		rm:         cfg.ResourceManager,
		updatedPod: make(chan *corev1.Pod, 100000),
		stopCh:     ctx.Done(),
	}, nil
}

func (p *PodAdapter) createConfigMaps(ctx context.Context, configmaps []string, ns string) error {
	for _, cm := range configmaps {
		_, err := p.clientCache.cmLister.ConfigMaps(ns).Get(cm)
		if err == nil {
			continue
		}
		if errors.IsNotFound(err) {
			configMap, err := p.rm.GetConfigMap(cm, ns)
			if err != nil {
				return fmt.Errorf("find comfigmap %v error %v", cm, err)
			}
			utils.TrimObjectMeta(&configMap.ObjectMeta)
			utils.SetObjectGlobal(&configMap.ObjectMeta)

			_, err = p.client.CoreV1().ConfigMaps(ns).Create(ctx, configMap, metav1.CreateOptions{})
			if err != nil {
				if errors.IsAlreadyExists(err) {
					continue
				}
				klog.Errorf("Failed to create configmap %v err: %v", cm, err)
				return err
			}
			klog.Infof("Create %v in %v success", cm, ns)
			continue
		}
		return fmt.Errorf("could not check configmap %s in external cluster: %v", cm, err)
	}
	return nil
}

func (p *PodAdapter) createPVCs(ctx context.Context, pvcs []string, ns string) error {
	for _, cm := range pvcs {
		_, err := p.client.CoreV1().PersistentVolumeClaims(ns).Get(ctx, cm, metav1.GetOptions{})
		if err == nil {
			continue
		}
		if errors.IsNotFound(err) {
			pvc, err := p.master.CoreV1().PersistentVolumeClaims(ns).Get(ctx, cm, metav1.GetOptions{})
			if err != nil {
				continue
			}
			utils.TrimObjectMeta(&pvc.ObjectMeta)
			utils.SetObjectGlobal(&pvc.ObjectMeta)
			_, err = p.client.CoreV1().PersistentVolumeClaims(ns).Create(ctx, pvc, metav1.CreateOptions{})
			if err != nil {
				if errors.IsAlreadyExists(err) {
					continue
				}
				klog.Errorf("Failed to create pvc %v err: %v", cm, err)
				return err
			}
			continue
		}
		return fmt.Errorf("could not check pvc %s in external cluster: %v", cm, err)
	}
	return nil
}

func (p *PodAdapter) createSA(ctx context.Context, sa string, ns string) (*corev1.ServiceAccount, error) {
	clientSA, err := p.client.CoreV1().ServiceAccounts(ns).Get(ctx, sa, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("could not check sa %s in member cluster: %v", sa, err)
	}

	if err == nil {
		return clientSA, nil
	}

	newSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sa,
			Namespace: ns,
		},
	}
	newSA, err = p.client.CoreV1().ServiceAccounts(ns).Create(ctx, newSA, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("could not create sa %s in member cluster: %v", sa, err)
	}

	return newSA, nil
}

func (p *PodAdapter) createSAToken(ctx context.Context, saName string, ns string) (*corev1.Secret, error) {
	sa, err := p.master.CoreV1().ServiceAccounts(ns).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not find sa %s in master cluster: %v", saName, err)
	}

	var secretName string
	if len(sa.Secrets) > 0 {
		secretName = sa.Secrets[0].Name
	}

	csName := fmt.Sprintf("master-%s-token", sa.Name)
	clientSecret, err := p.client.CoreV1().Secrets(ns).Get(ctx, csName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("could not check secret %s in member cluster: %v", secretName, err)
	}
	if err == nil {
		return clientSecret, nil
	}

	masterSecret, err := p.master.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not find secret %s in master cluster: %v", secretName, err)
	}

	nData := map[string][]byte{}
	nData["token"] = masterSecret.Data["token"]

	se := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csName,
			Namespace: ns,
		},
		Data: nData,
	}
	newSE, err := p.client.CoreV1().Secrets(ns).Create(ctx, se, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("could not create sa %s in member cluster: %v", sa, err)
	}
	return newSE, nil
}

func (p *PodAdapter) createCA(ctx context.Context, ns string) (*corev1.ConfigMap, error) {
	masterCA, err := p.client.CoreV1().ConfigMaps(ns).Get(ctx, MasterRooTCAName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("could not check configmap %s in member cluster: %v", MasterRooTCAName, err)
	}
	if err == nil {
		return masterCA, nil
	}

	ca, err := p.master.CoreV1().ConfigMaps(ns).Get(ctx, RooTCAConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not find configmap %s in master cluster: %v", ca, err)
	}

	newCA := ca.DeepCopy()
	newCA.Name = MasterRooTCAName
	utils.TrimObjectMeta(&newCA.ObjectMeta)

	newCA, err = p.client.CoreV1().ConfigMaps(ns).Create(ctx, newCA, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("could not create configmap %s in member cluster: %v", newCA.Name, err)
	}

	return newCA, nil
}

func (p *PodAdapter) convertAuth(ctx context.Context, pod *corev1.Pod) {
	if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		falseValue := false
		pod.Spec.AutomountServiceAccountToken = &falseValue

		sa := pod.Spec.ServiceAccountName
		_, err := p.createSA(ctx, sa, pod.Namespace)
		if err != nil {
			klog.Errorf("[convertAuth] create sa failed, ns: %s, pod: %s", pod.Namespace, pod.Name)
			return
		}

		se, err := p.createSAToken(ctx, sa, pod.Namespace)
		if err != nil {
			klog.Errorf("[convertAuth] create sa secret failed, ns: %s, pod: %s", pod.Namespace, pod.Name)
			return
		}

		rootCA, err := p.createCA(ctx, pod.Namespace)
		if err != nil {
			klog.Errorf("[convertAuth] create sa secret failed, ns: %s, pod: %s", pod.Namespace, pod.Name)
			return
		}

		volumes := pod.Spec.Volumes
		for _, v := range volumes {
			if strings.HasPrefix(v.Name, SATokenPrefix) {
				sources := []corev1.VolumeProjection{}
				for _, src := range v.Projected.Sources {
					if src.ServiceAccountToken != nil {
						continue
					}
					if src.ConfigMap != nil && src.ConfigMap.Name == RooTCAConfigMapName {
						src.ConfigMap.Name = rootCA.Name
					}
					sources = append(sources, src)
				}

				secretProjection := corev1.VolumeProjection{
					Secret: &corev1.SecretProjection{
						Items: []corev1.KeyToPath{
							{
								Key:  "token",
								Path: "token",
							},
						},
					},
				}
				secretProjection.Secret.Name = se.Name
				sources = append(sources, secretProjection)
				v.Projected.Sources = sources
			}
		}
	}
}

func (p *PodAdapter) createSecrets(ctx context.Context, secrets []string, ns string) error {
	for _, secretName := range secrets {
		_, err := p.clientCache.secretLister.Secrets(ns).Get(secretName)
		if err == nil {
			continue
		}
		if !errors.IsNotFound(err) {
			return err
		}
		secret, err := p.rm.GetSecret(secretName, ns)

		if err != nil {
			return err
		}
		utils.TrimObjectMeta(&secret.ObjectMeta)
		// skip service account secret
		if secret.Type == corev1.SecretTypeServiceAccountToken {
			if err := p.createServiceAccount(ctx, secret); err != nil {
				klog.Error(err)
				return err
			}
		}
		utils.SetObjectGlobal(&secret.ObjectMeta)
		_, err = p.client.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			klog.Errorf("Failed to create secret %v err: %v", secretName, err)
			return fmt.Errorf("could not create secret %s in external cluster: %v", secretName, err)
		}
	}
	return nil
}

func (p *PodAdapter) createServiceAccount(ctx context.Context, secret *corev1.Secret) error {
	if !p.enableServiceAccount {
		return nil
	}
	if secret.Annotations == nil {
		return fmt.Errorf("parse secret service account error")
	}
	klog.Infof("secret service-account info: [%v]", secret.Annotations)
	accountName := secret.Annotations[corev1.ServiceAccountNameKey]
	if accountName == "" {
		err := fmt.Errorf("get secret of serviceAccount not exits: [%s] [%v]",
			secret.Name, secret.Annotations)
		return err
	}

	ns := secret.Namespace
	sa, err := p.client.CoreV1().ServiceAccounts(ns).Get(ctx, accountName, metav1.GetOptions{})
	if err != nil || sa == nil {
		klog.Infof("get serviceAccount [%v] err: [%v]]", sa, err)
		sa, err = p.client.CoreV1().ServiceAccounts(ns).Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: accountName,
			},
		}, metav1.CreateOptions{})
		klog.Errorf("create serviceAccount [%v] err: [%v]", sa, err)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				return nil
			}
			return err
		}
	} else {
		klog.Infof("get secret serviceAccount info: [%s] [%v] [%v] [%v]",
			sa.Name, sa.CreationTimestamp, sa.Annotations, sa.UID)
	}
	secret.UID = sa.UID
	secret.Annotations[corev1.ServiceAccountNameKey] = accountName
	secret.Annotations[corev1.ServiceAccountUIDKey] = string(sa.UID)
	_, err = p.client.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		klog.Errorf("Failed to create secret %v err: %v", secret.Name, err)
	}

	sa.Secrets = []corev1.ObjectReference{{Name: secret.Name}}
	_, err = p.client.CoreV1().ServiceAccounts(ns).Update(ctx, sa, metav1.UpdateOptions{})
	if err != nil {
		klog.Infof(
			"update serviceAccount [%v] err: [%v]]",
			sa, err)
		return err
	}
	return nil
}

func (p *PodAdapter) Create(ctx context.Context, pod *corev1.Pod) error {
	if pod.Namespace == "kube-system" {
		return nil
	}
	basicPod := utils.TrimPod(pod, p.ignoreLabels)
	klog.Infof("Creating pod %v/%+v", pod.Namespace, pod.Name)
	if _, err := p.clientCache.nsLister.Get(pod.Namespace); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.Infof("Namespace %s does not exist for pod %s, creating it", pod.Namespace, pod.Name)
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: pod.Namespace,
			},
		}
		if _, createErr := p.client.CoreV1().Namespaces().Create(ctx, ns,
			metav1.CreateOptions{}); createErr != nil && errors.IsAlreadyExists(createErr) {
			klog.Infof("Namespace %s create failed error: %v", pod.Namespace, createErr)
			return err
		}
	}
	secretNames := getSecrets(pod)
	configMaps := getConfigmaps(pod)
	pvcs := getPVCs(pod)
	// nolint:errcheck
	go wait.PollImmediate(500*time.Millisecond, 10*time.Minute, func() (bool, error) {
		klog.Info("Trying to creating base dependent")
		if err := p.createConfigMaps(ctx, configMaps, pod.Namespace); err != nil {
			klog.Error(err)
			return false, nil
		}
		klog.Infof("Create configmaps %v of %v/%v success", configMaps, pod.Namespace, pod.Name)
		if err := p.createPVCs(ctx, pvcs, pod.Namespace); err != nil {
			klog.Error(err)
			return false, nil
		}
		klog.Infof("Create pvc %v of %v/%v success", pvcs, pod.Namespace, pod.Name)
		return true, nil
	})
	var err error
	// nolint:errcheck
	wait.PollImmediate(100*time.Millisecond, 1*time.Second, func() (bool, error) {
		klog.Info("Trying to creating secret and service account")
		if err = p.createSecrets(ctx, secretNames, pod.Namespace); err != nil {
			klog.Error(err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("create secrets failed: %v", err)
	}

	p.convertAuth(ctx, pod)

	klog.Infof("Creating pod %+v", pod)
	_, err = p.client.CoreV1().Pods(pod.Namespace).Create(ctx, basicPod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("could not create pod: %v", err)
	}
	klog.Infof("Create pod %v/%+v success", pod.Namespace, pod.Name)
	return nil
}

func (p *PodAdapter) Update(ctx context.Context, pod *corev1.Pod) error {
	if pod.Namespace == "kube-system" {
		return nil
	}
	klog.Infof("Updating pod %v/%+v", pod.Namespace, pod.Name)
	currentPod, err := p.Get(ctx, pod.Namespace, pod.Name)
	if err != nil {
		return fmt.Errorf("could not get current pod")
	}
	if !utils.IsVirtualPod(pod) {
		klog.Info("Pod is not created by vk, ignore")
		return nil
	}
	//tripped ignore labels which recoverd in currentPod
	utils.TrimLabels(currentPod.ObjectMeta.Labels, p.ignoreLabels)
	podCopy := currentPod.DeepCopy()
	// util.GetUpdatedPod update PodCopy container image, annotations, labels.
	// recover toleration, affinity, tripped ignore labels.
	utils.GetUpdatedPod(podCopy, pod, p.ignoreLabels)
	if reflect.DeepEqual(currentPod.Spec, podCopy.Spec) &&
		reflect.DeepEqual(currentPod.Annotations, podCopy.Annotations) &&
		reflect.DeepEqual(currentPod.Labels, podCopy.Labels) {
		return nil
	}
	_, err = p.client.CoreV1().Pods(pod.Namespace).Update(ctx, podCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("could not update pod: %v", err)
	}
	klog.Infof("Update pod %v/%+v success ", pod.Namespace, pod.Name)
	return nil
}

func (p *PodAdapter) Delete(ctx context.Context, pod *corev1.Pod) error {
	if pod.Namespace == "kube-system" {
		return nil
	}
	klog.Infof("Deleting pod %v/%+v", pod.Namespace, pod.Name)

	if !utils.IsVirtualPod(pod) {
		klog.Info("Pod is not create by vk, ignore")
		return nil
	}

	opts := &metav1.DeleteOptions{
		GracePeriodSeconds: new(int64), // 0
	}
	if pod.DeletionGracePeriodSeconds != nil {
		opts.GracePeriodSeconds = pod.DeletionGracePeriodSeconds
	}

	err := p.client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, *opts)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Tried to delete pod %s/%s, but it did not exist in the cluster", pod.Namespace, pod.Name)
			return nil
		}
		return fmt.Errorf("could not delete pod: %v", err)
	}
	klog.Infof("Delete pod %v/%+v success", pod.Namespace, pod.Name)
	return nil
}

func (p *PodAdapter) Get(ctx context.Context, namespace string, name string) (*corev1.Pod, error) {
	pod, err := p.clientCache.podLister.Pods(namespace).Get(name)
	if err != nil {
		klog.Error(err)
		if errors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("could not get pod %s/%s: %v", namespace, name, err)
	}
	podCopy := pod.DeepCopy()
	utils.RecoverLabels(podCopy.Labels, podCopy.Annotations)
	return podCopy, nil
}

func (p *PodAdapter) GetStatus(ctx context.Context, namespace string, name string) (*corev1.PodStatus, error) {
	pod, err := p.clientCache.podLister.Pods(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("could not get pod %s/%s: %v", namespace, name, err)
	}
	return pod.Status.DeepCopy(), nil
}

func (p *PodAdapter) List(_ context.Context) ([]*corev1.Pod, error) {
	set := labels.Set{utils.KosmosPodLabel: "true"}
	pods, err := p.clientCache.podLister.List(labels.SelectorFromSet(set))
	if err != nil {
		return nil, fmt.Errorf("could not list pods: %v", err)
	}

	podRefs := []*corev1.Pod{}
	for _, p := range pods {
		if !utils.IsVirtualPod(p) {
			continue
		}
		podCopy := p.DeepCopy()
		utils.RecoverLabels(podCopy.Labels, podCopy.Annotations)
		podRefs = append(podRefs, podCopy)
	}

	return podRefs, nil
}

func (p *PodAdapter) Notify(ctx context.Context, f func(*corev1.Pod)) {
	klog.Info("Called NotifyPods")
	go func() {
		// to make sure pods have been add to known pods
		time.Sleep(10 * time.Second)
		for {
			select {
			case pod := <-p.updatedPod:
				klog.Infof("Enqueue updated pod %v", pod.Name)
				// need trim pod, e.g. UID
				utils.RecoverLabels(pod.Labels, pod.Annotations)
				f(pod)
			case <-p.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

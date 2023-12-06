package rootpodsyncers

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/runtime"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

type K8sSyncer struct {
	RootClient         kubernetes.Interface
	GlobalLeafManager  leafUtils.LeafResourceManager
	EnvResourceManager utils.EnvResourceManager
	Options            *options.Options
	DynamicRootClient  dynamic.Interface
}

func (r *K8sSyncer) DeletePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootnamespacedname runtime.NamespacedName, cleanflag bool) error {
	klog.V(4).Infof("Deleting pod %v/%+v", rootnamespacedname.Namespace, rootnamespacedname.Name)
	// leafPod := &corev1.Pod{}

	cleanRootPodFunc := func() error {
		return DeletePodInRootCluster(ctx, rootnamespacedname, r.RootClient)
	}

	// leafpod, err := lr.Clientset.Get(ctx, rootnamespacedname, leafPod)
	leafPod, err := lr.Clientset.CoreV1().Pods(rootnamespacedname.Namespace).Get(ctx, rootnamespacedname.Name, metav1.GetOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			if cleanflag {
				return cleanRootPodFunc()
			}
			return nil
		}
		return err
	}

	if !podutils.IsKosmosPod(leafPod) {
		klog.V(4).Info("Pod is not create by kosmos tree, ignore")
		return nil
	}

	deleteOption := NewLeafDeleteOption(leafPod)
	err = lr.Client.Delete(ctx, leafPod, deleteOption)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("Tried to delete pod %s/%s, but it did not exist in the cluster", leafPod.Namespace, leafPod.Name)
			if cleanflag {
				return cleanRootPodFunc()
			}
			return nil
		}
		return fmt.Errorf("could not delete pod: %v", err)
	}
	klog.V(4).Infof("Delete pod %v/%+v success", leafPod.Namespace, leafPod.Name)
	return nil
}

func (r *K8sSyncer) CreatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, pod *corev1.Pod) error {
	if err := podutils.PopulateEnvironmentVariables(ctx, pod, r.EnvResourceManager); err != nil {
		// span.SetStatus(err)
		return err
	}

	clusterNodeInfo := r.GlobalLeafManager.GetClusterNode(pod.Spec.NodeName)
	if clusterNodeInfo == nil {
		return fmt.Errorf("clusternode info is nil , name: %s", pod.Spec.NodeName)
	}

	nodeSelector := r.GlobalLeafManager.GetClusterNode(pod.Spec.NodeName).LeafNodeSelector

	basicPod := podutils.FitPod(pod, lr.IgnoreLabels, clusterNodeInfo.LeafMode, nodeSelector)
	klog.V(4).Infof("Creating pod %v/%+v", pod.Namespace, pod.Name)

	// create ns
	ns := &corev1.Namespace{}
	nsKey := types.NamespacedName{
		Name: basicPod.Namespace,
	}
	if err := lr.Client.Get(ctx, nsKey, ns); err != nil {
		if !errors.IsNotFound(err) {
			// cannot get ns in root cluster, retry
			return err
		}
		klog.V(4).Infof("Namespace %s does not exist for pod %s, creating it", basicPod.Namespace, basicPod.Name)
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: basicPod.Namespace,
			},
		}

		if createErr := lr.Client.Create(ctx, ns); createErr != nil {
			if !errors.IsAlreadyExists(createErr) {
				klog.V(4).Infof("Namespace %s create failed error: %v", basicPod.Namespace, createErr)
				return err
			} else {
				// namespace already existed, skip create
				klog.V(4).Infof("Namespace %s already existed: %v", basicPod.Namespace, createErr)
			}
		}
	}

	if err := r.createVolumes(ctx, lr, basicPod, clusterNodeInfo); err != nil {
		klog.Errorf("Creating Volumes error %+v", basicPod)
		return err
	} else {
		klog.V(4).Infof("Creating Volumes successed %+v", basicPod)
	}

	r.convertAuth(ctx, lr, basicPod)

	if !r.Options.MultiClusterService {
		r.changeToMasterCoreDNS(ctx, basicPod, r.Options)
	}

	klog.V(4).Infof("Creating pod %+v", basicPod)

	err := lr.Client.Create(ctx, basicPod)
	if err != nil {
		return fmt.Errorf("could not create pod: %v", err)
	}
	klog.V(4).Infof("Create pod %v/%+v success", basicPod.Namespace, basicPod.Name)
	return nil
}

func (r *K8sSyncer) UpdatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootPod *corev1.Pod, leafPod *corev1.Pod) error {
	// TODO: update env
	// TODO： update config secret pv pvc ...
	klog.V(4).Infof("Updating pod %v/%+v", rootPod.Namespace, rootPod.Name)

	if !podutils.IsKosmosPod(leafPod) {
		klog.V(4).Info("Pod is not created by kosmos tree, ignore")
		return nil
	}
	// not used
	podutils.FitLabels(leafPod.ObjectMeta.Labels, lr.IgnoreLabels)
	podCopy := leafPod.DeepCopy()
	// util.GetUpdatedPod update PodCopy container image, annotations, labels.
	// recover toleration, affinity, tripped ignore labels.
	clusterNodeInfo := r.GlobalLeafManager.GetClusterNode(rootPod.Spec.NodeName)
	if clusterNodeInfo == nil {
		return fmt.Errorf("clusternode info is nil , name: %s", rootPod.Spec.NodeName)
	}

	nodeSelector := clusterNodeInfo.LeafNodeSelector

	podutils.GetUpdatedPod(podCopy, rootPod, lr.IgnoreLabels, clusterNodeInfo.LeafMode, nodeSelector)
	if reflect.DeepEqual(leafPod.Spec, podCopy.Spec) &&
		reflect.DeepEqual(leafPod.Annotations, podCopy.Annotations) &&
		reflect.DeepEqual(leafPod.Labels, podCopy.Labels) {
		return nil
	}

	r.convertAuth(ctx, lr, podCopy)

	if !r.Options.MultiClusterService {
		r.changeToMasterCoreDNS(ctx, podCopy, r.Options)
	}

	klog.V(4).Infof("Updating pod %+v", podCopy)

	err := lr.Client.Update(ctx, podCopy)
	if err != nil {
		return fmt.Errorf("could not update pod: %v", err)
	}
	klog.V(4).Infof("Update pod %v/%+v success ", rootPod.Namespace, rootPod.Name)
	return nil
}

func (r *K8sSyncer) GetPodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootnamespacedname runtime.NamespacedName) (*corev1.Pod, error) {
	leafPod := &corev1.Pod{}
	err := lr.Client.Get(ctx, types.NamespacedName{
		Namespace: rootnamespacedname.Namespace,
		Name:      rootnamespacedname.Name,
	}, leafPod)
	return leafPod, err
}

type rootDeleteOption struct {
	GracePeriodSeconds *int64
}

func (dopt *rootDeleteOption) ApplyToDelete(opt *client.DeleteOptions) {
	opt.GracePeriodSeconds = dopt.GracePeriodSeconds
}

func NewLeafDeleteOption(pod *corev1.Pod) client.DeleteOption {
	gracePeriodSeconds := new(int64)
	if pod.DeletionGracePeriodSeconds != nil {
		gracePeriodSeconds = pod.DeletionGracePeriodSeconds
	}

	return &rootDeleteOption{
		GracePeriodSeconds: gracePeriodSeconds,
	}
}

func DeletePodInRootCluster(ctx context.Context, rootnamespacedname runtime.NamespacedName, rootClient kubernetes.Interface) error {
	rPod, err := rootClient.CoreV1().Pods(rootnamespacedname.Namespace).Get(ctx, rootnamespacedname.Name, metav1.GetOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}

	rPodCopy := rPod.DeepCopy()

	if err := rootClient.CoreV1().Pods(rPodCopy.Namespace).Delete(ctx, rPodCopy.Name, metav1.DeleteOptions{
		GracePeriodSeconds: new(int64),
	}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (r *K8sSyncer) createStorageInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, gvr schema.GroupVersionResource, resourcenames []string, rootpod *corev1.Pod, cn *leafUtils.ClusterNode) error {
	ns := rootpod.Namespace
	storageHandler, err := NewStorageHandler(gvr)
	if err != nil {
		return err
	}
	for _, rname := range resourcenames {
		// add annotations for root
		rootobj, err := r.DynamicRootClient.Resource(gvr).Namespace(ns).Get(ctx, rname, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("could not get resource gvr(%v) %s from root cluster: %v", gvr, rname, err)
		}
		rootannotations := rootobj.GetAnnotations()
		rootannotations = utils.AddResourceClusters(rootannotations, lr.ClusterName)

		rootobj.SetAnnotations(rootannotations)

		_, err = r.DynamicRootClient.Resource(gvr).Namespace(ns).Update(ctx, rootobj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("could not update annotations of resource gvr(%v) %s from root cluster: %v", gvr, rname, err)
		}

		// create resource in leaf cluster
		_, err = lr.DynamicClient.Resource(gvr).Namespace(ns).Get(ctx, rname, metav1.GetOptions{})
		if err == nil {
			// already existed, so skip
			continue
		}
		if errors.IsNotFound(err) {
			unstructuredObj := rootobj

			podutils.FitUnstructuredObjMeta(unstructuredObj)

			if err := storageHandler.BeforeCreateInLeaf(ctx, r, lr, unstructuredObj, rootpod, cn); err != nil {
				return err
			}

			podutils.SetUnstructuredObjGlobal(unstructuredObj)

			_, err = lr.DynamicClient.Resource(gvr).Namespace(ns).Create(ctx, unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				if errors.IsAlreadyExists(err) {
					continue
				}
				klog.Errorf("Failed to create gvr(%v) %v err: %v", gvr, rname, err)
				return err
			}
			klog.V(4).Infof("Create gvr(%v) %v in %v success", gvr, rname, ns)
			continue
		}
		return fmt.Errorf("could not check gvr(%v) %s in external cluster: %v", gvr, rname, err)
	}
	return nil
}

func (r *K8sSyncer) createSAInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, sa string, ns string) (*corev1.ServiceAccount, error) {
	saKey := types.NamespacedName{
		Namespace: ns,
		Name:      sa,
	}

	clientSA := &corev1.ServiceAccount{}
	err := lr.Client.Get(ctx, saKey, clientSA)
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
	err = lr.Client.Create(ctx, newSA)
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("could not create sa %s in member cluster: %v", sa, err)
	}

	return newSA, nil
}

func (r *K8sSyncer) createSATokenInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, saName string, ns string) (*corev1.Secret, error) {
	satokenKey := types.NamespacedName{
		Namespace: ns,
		Name:      saName,
	}
	sa, err := r.RootClient.CoreV1().ServiceAccounts(satokenKey.Namespace).Get(ctx, satokenKey.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not find sa %s in master cluster: %v", saName, err)
	}

	var secretName string
	if len(sa.Secrets) > 0 {
		secretName = sa.Secrets[0].Name
	}

	csName := fmt.Sprintf("master-%s-token", sa.Name)
	csKey := types.NamespacedName{
		Namespace: ns,
		Name:      csName,
	}
	clientSecret := &corev1.Secret{}
	err = lr.Client.Get(ctx, csKey, clientSecret)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("could not check secret %s in member cluster: %v", secretName, err)
	}
	if err == nil {
		return clientSecret, nil
	}

	secretKey := types.NamespacedName{
		Namespace: ns,
		Name:      secretName,
	}

	masterSecret, err := r.RootClient.CoreV1().Secrets(secretKey.Namespace).Get(ctx, secretKey.Name, metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("could not find secret %s in master cluster: %v", secretName, err)
	}

	nData := map[string][]byte{}
	nData["token"] = masterSecret.Data["token"]

	newSE := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csName,
			Namespace: ns,
		},
		Data: nData,
	}
	err = lr.Client.Create(ctx, newSE)

	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("could not create sa %s in member cluster: %v", sa, err)
	}
	return newSE, nil
}

func (r *K8sSyncer) createCAInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, ns string) (*corev1.ConfigMap, error) {
	masterCAConfigmapKey := types.NamespacedName{
		Namespace: ns,
		Name:      utils.MasterRooTCAName,
	}

	masterCA := &corev1.ConfigMap{}

	err := lr.Client.Get(ctx, masterCAConfigmapKey, masterCA)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("could not check configmap %s in member cluster: %v", utils.MasterRooTCAName, err)
	}
	if err == nil {
		return masterCA, nil
	}

	ca, err := r.RootClient.CoreV1().ConfigMaps(ns).Get(ctx, utils.RooTCAConfigMapName, metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("could not find configmap %s in master cluster: %v", ca, err)
	}

	newCA := ca.DeepCopy()
	newCA.Name = utils.MasterRooTCAName
	podutils.FitObjectMeta(&newCA.ObjectMeta)

	err = lr.Client.Create(ctx, newCA)
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("could not create configmap %s in member cluster: %v", newCA.Name, err)
	}

	return newCA, nil
}

// changeToMasterCoreDNS point the dns of the pod to the master cluster, so that the pod can access any service.
// The master cluster holds all the services in the multi-cluster.
func (r *K8sSyncer) changeToMasterCoreDNS(ctx context.Context, pod *corev1.Pod, opts *options.Options) {
	if pod.Spec.DNSPolicy != corev1.DNSClusterFirst && pod.Spec.DNSPolicy != corev1.DNSClusterFirstWithHostNet {
		return
	}

	ns := pod.Namespace
	svc, err := r.RootClient.CoreV1().Services(opts.RootCoreDNSServiceNamespace).Get(ctx, opts.RootCoreDNSServiceName, metav1.GetOptions{})
	if err != nil {
		return
	}
	if svc != nil && svc.Spec.ClusterIP != "" {
		pod.Spec.DNSPolicy = "None"
		dnsConfig := corev1.PodDNSConfig{
			Nameservers: []string{
				svc.Spec.ClusterIP,
			},
			// TODO, if the master domain is changed, an exception will occur
			Searches: []string{
				fmt.Sprintf("%s.svc.cluster.local", ns),
				"svc.cluster.local",
				"cluster.local",
				"localdomain",
			},
		}
		pod.Spec.DNSConfig = &dnsConfig
	}
}

func (r *K8sSyncer) convertAuth(ctx context.Context, lr *leafUtils.LeafResource, pod *corev1.Pod) {
	if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		falseValue := false
		pod.Spec.AutomountServiceAccountToken = &falseValue

		sa := pod.Spec.ServiceAccountName
		_, err := r.createSAInLeafCluster(ctx, lr, sa, pod.Namespace)
		if err != nil {
			klog.Errorf("[convertAuth] create sa failed, ns: %s, pod: %s", pod.Namespace, pod.Name)
			return
		}

		se, err := r.createSATokenInLeafCluster(ctx, lr, sa, pod.Namespace)
		if err != nil {
			klog.Errorf("[convertAuth] create sa secret failed, ns: %s, pod: %s", pod.Namespace, pod.Name)
			return
		}

		rootCA, err := r.createCAInLeafCluster(ctx, lr, pod.Namespace)
		if err != nil {
			klog.Errorf("[convertAuth] create sa secret failed, ns: %s, pod: %s", pod.Namespace, pod.Name)
			return
		}

		volumes := pod.Spec.Volumes
		for _, v := range volumes {
			if strings.HasPrefix(v.Name, utils.SATokenPrefix) {
				sources := []corev1.VolumeProjection{}
				for _, src := range v.Projected.Sources {
					if src.ServiceAccountToken != nil {
						continue
					}
					if src.ConfigMap != nil && src.ConfigMap.Name == utils.RooTCAConfigMapName {
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

func (r *K8sSyncer) createServiceAccountInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, secret *corev1.Secret) error {
	if !lr.EnableServiceAccount {
		return nil
	}
	if secret.Annotations == nil {
		return fmt.Errorf("parse secret service account error")
	}
	klog.V(4).Infof("secret service-account info: [%v]", secret.Annotations)
	accountName := secret.Annotations[corev1.ServiceAccountNameKey]
	if accountName == "" {
		err := fmt.Errorf("get secret of serviceAccount not exits: [%s] [%v]",
			secret.Name, secret.Annotations)
		return err
	}

	ns := secret.Namespace
	sa := &corev1.ServiceAccount{}
	saKey := types.NamespacedName{
		Namespace: ns,
		Name:      accountName,
	}

	err := lr.Client.Get(ctx, saKey, sa)
	if err != nil || sa == nil {
		klog.V(4).Infof("get serviceAccount [%v] err: [%v]]", sa, err)
		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      accountName,
				Namespace: ns,
			},
		}
		err := lr.Client.Create(ctx, sa)
		klog.Errorf("create serviceAccount [%v] err: [%v]", sa, err)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				return nil
			}
			return err
		}
	} else {
		klog.V(4).Infof("get secret serviceAccount info: [%s] [%v] [%v] [%v]",
			sa.Name, sa.CreationTimestamp, sa.Annotations, sa.UID)
	}
	secret.UID = sa.UID
	secret.Annotations[corev1.ServiceAccountNameKey] = accountName
	secret.Annotations[corev1.ServiceAccountUIDKey] = string(sa.UID)

	secret.ObjectMeta.Namespace = ns

	err = lr.Client.Create(ctx, secret)

	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		klog.Errorf("Failed to create secret %v err: %v", secret.Name, err)
	}

	sa.Secrets = []corev1.ObjectReference{{Name: secret.Name}}

	err = lr.Client.Update(ctx, sa)
	if err != nil {
		klog.V(4).Infof(
			"update serviceAccount [%v] err: [%v]]",
			sa, err)
		return err
	}
	return nil
}

func (r *K8sSyncer) createVolumes(ctx context.Context, lr *leafUtils.LeafResource, basicPod *corev1.Pod, clusterNodeInfo *leafUtils.ClusterNode) error {
	// create secret configmap pvc
	secretNames, imagePullSecrets := podutils.GetSecrets(basicPod)
	configMaps := podutils.GetConfigmaps(basicPod)
	pvcs := podutils.GetPVCs(basicPod)

	ch := make(chan string, 3)

	// configmap
	go func() {
		if err := wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
			klog.V(4).Info("Trying to creating dependent configmaps")
			if err := r.createStorageInLeafCluster(ctx, lr, utils.GVR_CONFIGMAP, configMaps, basicPod, clusterNodeInfo); err != nil {
				klog.Error(err)
				return false, nil
			}
			klog.V(4).Infof("Create configmaps %v of %v/%v success", configMaps, basicPod.Namespace, basicPod.Name)
			return true, nil
		}); err != nil {
			ch <- fmt.Sprintf("create configmap failed: %v", err)
		}
		ch <- ""
	}()

	// pvc
	go func() {
		if err := wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
			if !r.Options.OnewayStorageControllers {
				klog.V(4).Info("Trying to creating dependent pvc")
				if err := r.createStorageInLeafCluster(ctx, lr, utils.GVR_PVC, pvcs, basicPod, clusterNodeInfo); err != nil {
					klog.Error(err)
					return false, nil
				}
				klog.V(4).Infof("Create pvc %v of %v/%v success", pvcs, basicPod.Namespace, basicPod.Name)
			}
			return true, nil
		}); err != nil {
			ch <- fmt.Sprintf("create pvc failed: %v", err)
		}
		ch <- ""
	}()

	// secret
	go func() {
		if err := wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
			klog.V(4).Info("Trying to creating secret")
			if err := r.createStorageInLeafCluster(ctx, lr, utils.GVR_SECRET, secretNames, basicPod, clusterNodeInfo); err != nil {
				klog.Error(err)
				return false, nil
			}

			// try to create image pull secrets, ignore err
			if errignore := r.createStorageInLeafCluster(ctx, lr, utils.GVR_SECRET, imagePullSecrets, basicPod, clusterNodeInfo); errignore != nil {
				klog.Warning(errignore)
			}
			return true, nil
		}); err != nil {
			ch <- fmt.Sprintf("create secrets failed: %v", err)
		}
		ch <- ""
	}()

	t1 := <-ch
	t2 := <-ch
	t3 := <-ch

	errString := ""
	errs := []string{t1, t2, t3}
	for i := range errs {
		if len(errs[i]) > 0 {
			errString = errString + errs[i]
		}
	}

	if len(errString) > 0 {
		return fmt.Errorf("%s", errString)
	}

	return nil
}

package pod

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/extensions/daemonset"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/convertpolicy"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

const (
	RootPodControllerName = "root-pod-controller"
)

type RootPodReconciler struct {
	client.Client
	RootClient client.Client

	DynamicRootClient  dynamic.Interface
	envResourceManager utils.EnvResourceManager

	GlobalLeafManager leafUtils.LeafResourceManager

	Options *options.Options
}

type envResourceManager struct {
	DynamicRootClient dynamic.Interface
}

// GetConfigMap retrieves the specified config map from the cache.
func (rm *envResourceManager) GetConfigMap(name, namespace string) (*corev1.ConfigMap, error) {
	// return rm.configMapLister.ConfigMaps(namespace).Get(name)
	obj, err := rm.DynamicRootClient.Resource(utils.GVR_CONFIGMAP).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	retObj := &corev1.ConfigMap{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &retObj); err != nil {
		return nil, err
	}

	return retObj, nil
}

// GetSecret retrieves the specified secret from Kubernetes.
func (rm *envResourceManager) GetSecret(name, namespace string) (*corev1.Secret, error) {
	// return rm.secretLister.Secrets(namespace).Get(name)
	obj, err := rm.DynamicRootClient.Resource(utils.GVR_SECRET).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	retObj := &corev1.Secret{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &retObj); err != nil {
		return nil, err
	}

	return retObj, nil
}

// ListServices retrieves the list of services from Kubernetes.
func (rm *envResourceManager) ListServices() ([]*corev1.Service, error) {
	// return rm.serviceLister.List(labels.Everything())
	objs, err := rm.DynamicRootClient.Resource(utils.GVR_SERVICE).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	})

	if err != nil {
		return nil, err
	}

	retObj := make([]*corev1.Service, 0)

	for _, obj := range objs.Items {
		tmpObj := &corev1.Service{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &tmpObj); err != nil {
			return nil, err
		}
		retObj = append(retObj, tmpObj)
	}

	return retObj, nil
}

func NewEnvResourceManager(client dynamic.Interface) utils.EnvResourceManager {
	return &envResourceManager{
		DynamicRootClient: client,
	}
}

func (r *RootPodReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var cachepod corev1.Pod
	if err := r.Get(ctx, request.NamespacedName, &cachepod); err != nil {
		if errors.IsNotFound(err) {
			// TODO: we cannot get leaf pod when we donnot known the node name of pod, so delete all ...
			nodeNames := r.GlobalLeafManager.ListNodes()
			for _, nodeName := range nodeNames {
				lr, err := r.GlobalLeafManager.GetLeafResourceByNodeName(nodeName)
				if err != nil {
					// wait for leaf resource init
					return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
				}
				if err := r.DeletePodInLeafCluster(ctx, lr, request.NamespacedName, false); err != nil {
					klog.Errorf("delete pod in leaf error[1]: %v,  %s", err, request.NamespacedName)
					return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
				}
			}
			return reconcile.Result{}, nil
		}
		klog.Errorf("get %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	rootpod := *(cachepod.DeepCopy())

	// node filter
	if !strings.HasPrefix(rootpod.Spec.NodeName, utils.KosmosNodePrefix) {
		// ignore the pod who donnot has the annotations "kosmos-io/owned-by-cluster"
		// TODO： use const
		nn := types.NamespacedName{
			Namespace: "",
			Name:      rootpod.Spec.NodeName,
		}

		targetNode := &corev1.Node{}
		if err := r.RootClient.Get(ctx, nn, targetNode); err != nil {
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}

		if targetNode.Annotations == nil {
			return reconcile.Result{}, nil
		}

		clusterName := targetNode.Annotations[utils.KosmosNodeOwnedByClusterAnnotations]

		if len(clusterName) == 0 {
			return reconcile.Result{}, nil
		}
	}

	// TODO: GlobalLeafResourceManager may not inited....
	// belongs to the current node
	if !r.GlobalLeafManager.HasNode(rootpod.Spec.NodeName) {
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	lr, err := r.GlobalLeafManager.GetLeafResourceByNodeName(rootpod.Spec.NodeName)
	if err != nil {
		// wait for leaf resource init
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	// skip namespace
	if len(lr.Namespace) > 0 && lr.Namespace != rootpod.Namespace {
		return reconcile.Result{}, nil
	}

	// delete pod in leaf
	if !rootpod.GetDeletionTimestamp().IsZero() {
		if err := r.DeletePodInLeafCluster(ctx, lr, request.NamespacedName, true); err != nil {
			klog.Errorf("delete pod in leaf error[1]: %v,  %s", err, request.NamespacedName)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
		return reconcile.Result{}, nil
	}

	leafPod := &corev1.Pod{}
	err = lr.Client.Get(ctx, request.NamespacedName, leafPod)

	// create pod in leaf
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.CreatePodInLeafCluster(ctx, lr, &rootpod, r.GlobalLeafManager.GetClusterNode(rootpod.Spec.NodeName).LeafNodeSelector); err != nil {
				klog.Errorf("create pod inleaf error, err: %s", err)
				return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
			} else {
				return reconcile.Result{}, nil
			}
		} else {
			klog.Errorf("get pod in leaf error[3]: %v,  %s", err, request.NamespacedName)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
	}

	// update pod in leaf
	if podutils.ShouldEnqueue(leafPod, &rootpod) {
		if err := r.UpdatePodInLeafCluster(ctx, lr, &rootpod, leafPod, r.GlobalLeafManager.GetClusterNode(rootpod.Spec.NodeName).LeafNodeSelector); err != nil {
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
	}

	return reconcile.Result{}, nil
}

func (r *RootPodReconciler) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	r.envResourceManager = NewEnvResourceManager(r.DynamicRootClient)

	skipFunc := func(obj client.Object) bool {
		// skip reservedNS
		if obj.GetNamespace() == utils.ReservedNS {
			return false
		}
		// don't create pod if pod has label daemonset.kosmos.io/managed=""
		if _, ok := obj.GetLabels()[daemonset.ManagedLabel]; ok {
			return false
		}

		p := obj.(*corev1.Pod)

		// skip daemonset
		if p.OwnerReferences != nil && len(p.OwnerReferences) > 0 {
			for _, or := range p.OwnerReferences {
				if or.Kind == "DaemonSet" {
					if p.Annotations != nil {
						if _, ok := p.Annotations[utils.KosmosDaemonsetAllowAnnotations]; ok {
							return true
						}
					}
					if !p.DeletionTimestamp.IsZero() {
						return true
					}
					return false
				}
			}
		}
		return true
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(RootPodControllerName).
		WithOptions(controller.Options{}).
		For(&corev1.Pod{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return skipFunc(createEvent.Object)
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return skipFunc(updateEvent.ObjectNew)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return skipFunc(deleteEvent.Object)
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				// TODO
				return false
			},
		})).
		Complete(r)
}

func (r *RootPodReconciler) createStorageInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, gvr schema.GroupVersionResource, resourcenames []string, rootpod *corev1.Pod, cn *leafUtils.ClusterNode) error {
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
		unstructuredObj := rootobj

		podutils.FitUnstructuredObjMeta(unstructuredObj)
		if err := storageHandler.BeforeGetInLeaf(ctx, r, lr, unstructuredObj, rootpod, cn); err != nil {
			return err
		}

		_, err = lr.DynamicClient.Resource(gvr).Namespace(ns).Get(ctx, unstructuredObj.GetName(), metav1.GetOptions{})
		if err == nil {
			// already existed, so skip
			continue
		}
		if errors.IsNotFound(err) {
			podutils.SetUnstructuredObjGlobal(unstructuredObj)
			if err := storageHandler.BeforeCreateInLeaf(ctx, r, lr, unstructuredObj, rootpod, cn); err != nil {
				return err
			}

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

func (r *RootPodReconciler) createSATokenInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, saName string, pod *corev1.Pod) (string, error) {
	// get the token-secret of sa
	ns := pod.Namespace
	satokenKey := types.NamespacedName{
		Namespace: ns,
		Name:      saName,
	}
	sa := &corev1.ServiceAccount{}
	err := r.RootClient.Get(ctx, satokenKey, sa)
	if err != nil {
		return "", fmt.Errorf("could not find sa %s in master cluster: %v", saName, err)
	}

	var rootSecretName string
	if len(sa.Secrets) > 0 {
		rootSecretName = sa.Secrets[0].Name
	}

	if rootSecretName == "" {
		// k8s version >=1.24, sa does not create token-secret by default,
		// so we will create and mount it to the subset group
		tokenSecret, err := r.createTokenSecretInRootCluster(ctx, sa)
		if err != nil {
			return "", err
		}

		rootSecretName = tokenSecret.Name
	}

	csKey := types.NamespacedName{
		Namespace: ns,
		Name:      rootSecretName,
	}
	clientSecret := &corev1.Secret{}
	err = lr.Client.Get(ctx, csKey, clientSecret)
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("could not check secret %s in member cluster: %v", csKey.Name, err)
	}
	klog.V(4).Infof("get secret %v from client", clientSecret)
	if err == nil {
		return clientSecret.Name, nil
	}

	// this secret needs to be created in member cluster
	ch := make(chan string, 1)
	clusterNodeInfo := r.GlobalLeafManager.GetClusterNode(pod.Spec.NodeName)
	go func() {
		if err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
			if err := r.createStorageInLeafCluster(ctx, lr, utils.GVR_SECRET, []string{rootSecretName}, pod, clusterNodeInfo); err == nil {
				klog.Info("create secret rootSecretName in leaf cluster success")
				return true, nil
			} else {
				return false, err
			}
		}); err != nil {
			ch <- fmt.Sprintf("could not create secret token %s in leaf cluster: %v", rootSecretName, err)
		}
		ch <- ""
	}()

	t := <-ch
	errString := "" + t
	if len(errString) > 0 {
		return "", fmt.Errorf("%s", errString)
	}

	return rootSecretName, nil
}

func (r *RootPodReconciler) createTokenSecretInRootCluster(ctx context.Context, sa *corev1.ServiceAccount) (*corev1.Secret, error) {
	tokenSecretName := fmt.Sprintf("kosmos-%s-token", sa.Name)
	tokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenSecretName,
			Namespace: sa.Namespace,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: sa.Name,
				corev1.ServiceAccountUIDKey:  string(sa.UID),
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	err := r.RootClient.Create(ctx, tokenSecret)

	if err != nil {
		return nil, fmt.Errorf("could not create token-secret %s in host cluster: %v", tokenSecretName, err)
	}

	// Attach token-secret to sa
	patchSa := sa.DeepCopy()
	patchSa.Secrets = []corev1.ObjectReference{
		{
			Name: tokenSecretName,
		},
	}
	err = r.RootClient.Update(ctx, patchSa)
	if err != nil {
		return nil, fmt.Errorf("could not update sa %s in host cluster: %v", patchSa.Name, err)
	}

	return tokenSecret, nil
}

// createConfigMapInLeafCluster create cm in leaf cluster
func (r *RootPodReconciler) createConfigMapInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, configMapName string, pod *corev1.Pod) (string, error) {
	ns := pod.Namespace
	memberConfigmapKeyName := configMapName

	// The name of the host cluster kube-root-ca.crt in the leaf group is master-root-ca.crt
	if strings.HasPrefix(configMapName, utils.RooTCAConfigMapName) {
		memberConfigmapKeyName = utils.MasterRooTCAName
	}

	configmapKey := types.NamespacedName{
		Namespace: ns,
		Name:      memberConfigmapKeyName,
	}

	memberConfigMap := &corev1.ConfigMap{}

	err := lr.Client.Get(ctx, configmapKey, memberConfigMap)
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("could not check configmap %s in member cluster: %v", configmapKey.Name, err)
	}
	if err == nil {
		return memberConfigMap.Name, nil
	}

	ch := make(chan string, 1)
	clusterNodeInfo := r.GlobalLeafManager.GetClusterNode(pod.Spec.NodeName)
	go func() {
		if err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
			if err = r.createStorageInLeafCluster(ctx, lr, utils.GVR_CONFIGMAP, []string{configMapName}, pod, clusterNodeInfo); err == nil {
				return true, nil
			} else {
				return false, err
			}
		}); err != nil {
			ch <- fmt.Sprintf("could not create configmap %s in member cluster: %v", configMapName, err)
		}
		ch <- ""
	}()

	t := <-ch
	errString := "" + t
	if len(errString) > 0 {
		return "", fmt.Errorf("%s", errString)
	}

	return memberConfigmapKeyName, nil
}

func (r *RootPodReconciler) createSecretInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, secretName string, pod *corev1.Pod) (string, error) {
	ns := pod.Namespace
	secretKey := types.NamespacedName{
		Namespace: ns,
		Name:      secretName,
	}

	memberSecret := &corev1.Secret{}

	err := lr.Client.Get(ctx, secretKey, memberSecret)
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("could not check secret %s in member cluster: %v", secretKey.Name, err)
	}
	if err == nil {
		return memberSecret.Name, nil
	}

	ch := make(chan string, 1)
	clusterNodeInfo := r.GlobalLeafManager.GetClusterNode(pod.Spec.NodeName)
	go func() {
		if err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
			if err = r.createStorageInLeafCluster(ctx, lr, utils.GVR_SECRET, []string{secretName}, pod, clusterNodeInfo); err == nil {
				return true, nil
			} else {
				return false, err
			}
		}); err != nil {
			ch <- fmt.Sprintf("could not create secret %s in member cluster: %v", secretName, err)
		}
		ch <- ""
	}()

	t := <-ch
	errString := "" + t
	if len(errString) > 0 {
		return "", fmt.Errorf("%s", errString)
	}

	return secretName, nil
}

// changeToMasterCoreDNS point the dns of the pod to the master cluster, so that the pod can access any service.
// The master cluster holds all the services in the multi-cluster.
func (r *RootPodReconciler) changeToMasterCoreDNS(ctx context.Context, pod *corev1.Pod, opts *options.Options) {
	if pod.Spec.DNSPolicy != corev1.DNSClusterFirst && pod.Spec.DNSPolicy != corev1.DNSClusterFirstWithHostNet {
		return
	}

	ns := pod.Namespace
	svc := &corev1.Service{}
	err := r.RootClient.Get(ctx, types.NamespacedName{Namespace: opts.RootCoreDNSServiceNamespace, Name: opts.RootCoreDNSServiceName}, svc)
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

// projectedHandler Process the project volume, creating and mounting secret, configmap, DownwardAPI,
// and ServiceAccountToken from the project volume in the member cluster to the pod of the host cluster
func (r *RootPodReconciler) projectedHandler(ctx context.Context, lr *leafUtils.LeafResource, pod *corev1.Pod) {
	falseValue := false
	pod.Spec.AutomountServiceAccountToken = &falseValue

	if len(pod.Spec.Volumes) == 0 {
		return
	}

	for _, volume := range pod.Spec.Volumes {
		if volume.Projected != nil {
			saName := pod.Spec.ServiceAccountName
			var sources []corev1.VolumeProjection

			for _, projectedVolumeSource := range volume.Projected.Sources {
				// Process all resources for the rootpod
				if projectedVolumeSource.ServiceAccountToken != nil {
					tokenSecretName, err := r.createSATokenInLeafCluster(ctx, lr, saName, pod)
					if err != nil {
						klog.Errorf("[convertAuth] create sa secret failed, ns: %s, pod: %s, err: %s", pod.Namespace, pod.Name, err)
						return
					}
					secretProjection := corev1.VolumeProjection{
						Secret: &corev1.SecretProjection{
							Items: []corev1.KeyToPath{
								{
									Key:  "token",
									Path: projectedVolumeSource.ServiceAccountToken.Path,
								},
							},
						},
					}
					secretProjection.Secret.Name = tokenSecretName
					sources = append(sources, secretProjection)
				}
				if projectedVolumeSource.ConfigMap != nil {
					cmName, err := r.createConfigMapInLeafCluster(ctx, lr, projectedVolumeSource.ConfigMap.Name, pod)
					if err != nil {
						klog.Errorf("[convertAuth] create configmap failed, ns: %s, cm: %s, err: %s", pod.Namespace, cmName, err)
						return
					}
					cmDeepCopy := projectedVolumeSource.DeepCopy()
					cmDeepCopy.ConfigMap.Name = cmName
					sources = append(sources, *cmDeepCopy)
				}
				if projectedVolumeSource.Secret != nil {
					Secret := projectedVolumeSource.Secret
					seName, err := r.createSecretInLeafCluster(ctx, lr, Secret.Name, pod)
					if err != nil {
						klog.Errorf("[convertAuth] create secret failed, ns: %s, cm: %s, err: %s", pod.Namespace, seName, err)
						return
					}
					secretDeepCopy := projectedVolumeSource.DeepCopy()
					secretDeepCopy.Secret.Name = seName
					sources = append(sources, *secretDeepCopy)
				}
				if projectedVolumeSource.DownwardAPI != nil {
					DownwardAPIProjection := corev1.VolumeProjection{
						DownwardAPI: projectedVolumeSource.DownwardAPI,
					}
					sources = append(sources, DownwardAPIProjection)
				}
			}
			volume.Projected.Sources = sources
			klog.V(4).Infof("volume.Projected.Sources: %v", sources)
		}
	}
}

// createServiceAccountInLeafCluster Create an sa corresponding to token-secret in member cluster
func (r *RootPodReconciler) createServiceAccountInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, secret *corev1.Secret) error {
	if !lr.EnableServiceAccount {
		return nil
	}
	if secret.Annotations == nil {
		return fmt.Errorf("parse secret service account error")
	}
	klog.V(4).Infof("secret service-account info: [%v]", secret.Annotations)
	accountName := secret.Annotations[corev1.ServiceAccountNameKey]
	if accountName == "" {
		err := fmt.Errorf("get serviceAccount of secret not exits: [%s] [%v]",
			secret.Name, secret.Annotations)
		return err
	}

	sa := &corev1.ServiceAccount{}
	if accountName != utils.DefaultServiceAccountName {
		ns := secret.Namespace
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
			klog.Infof("create serviceAccount [%v] err: [%v] ", sa, err)
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
	} else {
		// accountName == default
		// Type set Opaque and add annotation of kosmos.io/service-account.name
		secret.Annotations[utils.DefaultServiceAccountToken] = utils.DefaultServiceAccountName
		secret.Type = corev1.SecretTypeOpaque
	}

	err := lr.Client.Create(ctx, secret)

	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		klog.Errorf("Failed to create secret %v err: %v", secret.Name, err)
	}

	// the secret-token cannot be mounted to the default-sa of the leaf cluster
	if accountName != utils.DefaultServiceAccountName {
		saCopy := sa.DeepCopy()
		err := updateServiceAccountObjectReferenceRetry(ctx, saCopy, lr, secret.Name)
		if err != nil {
			klog.Errorf("update serviceAccount [%v] err: [%v]]", saCopy, err)
			return err
		}
	}

	return nil
}

// nolint:dupl
func updateServiceAccountObjectReferenceRetry(ctx context.Context, saCopy *corev1.ServiceAccount, lr *leafUtils.LeafResource, secretName string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		saCopy.Secrets = []corev1.ObjectReference{{Name: secretName}}
		err := lr.Client.Update(ctx, saCopy)
		if err == nil {
			return nil
		}

		klog.Errorf("Failed to update ServiceAccountObjectReference %s/%s - secret %s: %v", saCopy.Namespace, saCopy.Name, secretName, err)
		newServiceAccount := &corev1.ServiceAccount{}
		err = lr.Client.Get(ctx, client.ObjectKey{Namespace: saCopy.Namespace, Name: saCopy.Name}, newServiceAccount)
		if err == nil {
			//Make a copy, so we don't mutate the shared cache
			saCopy = newServiceAccount.DeepCopy()
		} else {
			klog.Errorf("Failed to get ServiceAccount %s/%s: %v", saCopy.Namespace, saCopy.Name, err)
		}

		return err
	})
}

func (r *RootPodReconciler) createVolumes(ctx context.Context, lr *leafUtils.LeafResource, basicPod *corev1.Pod, clusterNodeInfo *leafUtils.ClusterNode) error {
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
			pvcsWithoutEs, err := podutils.NoOneWayPVCFilter(ctx, r.DynamicRootClient, pvcs, basicPod.Namespace)
			if err != nil {
				klog.Error(err)
				return false, err
			}
			klog.V(4).Info("Trying to creating dependent pvc")
			if err := r.createStorageInLeafCluster(ctx, lr, utils.GVR_PVC, pvcsWithoutEs, basicPod, clusterNodeInfo); err != nil {
				klog.Error(err)
				return false, nil
			}
			klog.V(4).Infof("Create pvc %v of %v/%v success", pvcsWithoutEs, basicPod.Namespace, basicPod.Name)
			// }
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

// mutatePod modify pod by matching policy
func (r *RootPodReconciler) mutatePod(ctx context.Context, pod *corev1.Pod, nodeName string) error {
	klog.V(4).Infof("Converting pod %v/%+v", pod.Namespace, pod.Name)

	cpcpList := &kosmosv1alpha1.ClusterPodConvertPolicyList{}
	pcpList := &kosmosv1alpha1.PodConvertPolicyList{}
	err := r.Client.List(ctx, cpcpList, &client.ListOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Infof("list cluster pod convert policy error: %v", err)
	} else {
		err = r.Client.List(ctx, pcpList, &client.ListOptions{
			Namespace: pod.Namespace,
		})
		if err != nil && !errors.IsNotFound(err) {
			klog.Infof("list pod convert policy error: %v", err)
		}
	}

	if len(cpcpList.Items) <= 0 && len(pcpList.Items) <= 0 {
		// no matched policy, skip
		return nil
	}

	rootNode := &corev1.Node{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: nodeName}, rootNode)
	if err != nil {
		return fmt.Errorf("get node error: %v, nodeName: %s", err, pod.Spec.NodeName)
	}

	if len(cpcpList.Items) > 0 {
		matchedPolicy, err := convertpolicy.GetMatchClusterPodConvertPolicy(*cpcpList, pod.Labels, rootNode.Labels)
		if err != nil {
			return fmt.Errorf("get pod convert policy error: %v", err)
		}
		podutils.ConvertPod(pod, matchedPolicy, nil)
	} else {
		matchedPolicy, err := convertpolicy.GetMatchPodConvertPolicy(*pcpList, pod.Labels, rootNode.Labels)
		if err != nil {
			return fmt.Errorf("get pod convert policy error: %v", err)
		}
		podutils.ConvertPod(pod, nil, matchedPolicy)
	}

	klog.V(4).Infof("Convert pod %v/%+v success", pod.Namespace, pod.Name)
	return nil
}

func (r *RootPodReconciler) CreatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, pod *corev1.Pod, nodeSelector kosmosv1alpha1.NodeSelector) error {
	if err := podutils.PopulateEnvironmentVariables(ctx, pod, r.envResourceManager); err != nil {
		// span.SetStatus(err)
		return err
	}

	clusterNodeInfo := r.GlobalLeafManager.GetClusterNode(pod.Spec.NodeName)
	if clusterNodeInfo == nil {
		return fmt.Errorf("clusternode info is nil , name: %s", pod.Spec.NodeName)
	}

	basicPod := podutils.FitPod(pod, lr.IgnoreLabels, clusterNodeInfo.LeafMode, nodeSelector)
	klog.V(4).Infof("Creating pod %v/%+v", pod.Namespace, pod.Name)

	err := r.mutatePod(ctx, basicPod, pod.Spec.NodeName)
	if err != nil {
		klog.Errorf("Converting pod error: %v", err)
	}

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
				klog.V(4).Info("Namespace %s already existed: %v", basicPod.Namespace, createErr)
			}
		}
	}

	if err := r.createVolumes(ctx, lr, basicPod, clusterNodeInfo); err != nil {
		klog.Errorf("Creating Volumes error %+v", basicPod)
		return err
	} else {
		klog.V(4).Infof("Creating Volumes successed %+v", basicPod)
	}

	r.projectedHandler(ctx, lr, basicPod)

	if !r.Options.MultiClusterService {
		r.changeToMasterCoreDNS(ctx, basicPod, r.Options)
	}

	klog.V(4).Infof("Creating pod %+v", basicPod)

	err = lr.Client.Create(ctx, basicPod)
	if err != nil {
		return fmt.Errorf("could not create pod: %v", err)
	}
	klog.V(4).Infof("Create pod %v/%+v success", basicPod.Namespace, basicPod.Name)
	return nil
}

func (r *RootPodReconciler) UpdatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootPod *corev1.Pod, leafPod *corev1.Pod, nodeSelector kosmosv1alpha1.NodeSelector) error {
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
	podutils.GetUpdatedPod(podCopy, rootPod, lr.IgnoreLabels, clusterNodeInfo.LeafMode, nodeSelector)
	if reflect.DeepEqual(leafPod.Spec, podCopy.Spec) &&
		reflect.DeepEqual(leafPod.Annotations, podCopy.Annotations) &&
		reflect.DeepEqual(leafPod.Labels, podCopy.Labels) {
		return nil
	}

	r.projectedHandler(ctx, lr, podCopy)

	if !r.Options.MultiClusterService {
		r.changeToMasterCoreDNS(ctx, podCopy, r.Options)
	}

	klog.V(5).Infof("Updating pod %+v", podCopy)

	err := lr.Client.Update(ctx, podCopy)
	if err != nil {
		return fmt.Errorf("could not update pod: %v", err)
	}
	klog.V(4).Infof("Update pod %v/%+v success ", rootPod.Namespace, rootPod.Name)
	return nil
}

func (r *RootPodReconciler) DeletePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootnamespacedname types.NamespacedName, cleanflag bool) error {
	klog.V(4).Infof("Deleting pod %v/%+v", rootnamespacedname.Namespace, rootnamespacedname.Name)
	leafPod := &corev1.Pod{}

	cleanRootPodFunc := func() error {
		return DeletePodInRootCluster(ctx, rootnamespacedname, r.Client)
	}

	err := lr.Client.Get(ctx, rootnamespacedname, leafPod)

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

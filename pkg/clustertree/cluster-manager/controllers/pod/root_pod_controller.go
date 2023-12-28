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
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/extensions/daemonset"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

const (
	RootPodControllerName = "root-pod-controller"
	RootPodRequeueTime    = 10 * time.Second
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
					return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
				}
				if err := r.DeletePodInLeafCluster(ctx, lr, request.NamespacedName, false); err != nil {
					klog.Errorf("delete pod in leaf error[1]: %v,  %s", err, request.NamespacedName)
					return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
				}
			}
			return reconcile.Result{}, nil
		}
		klog.Errorf("get %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
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
			return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
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
		return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
	}

	lr, err := r.GlobalLeafManager.GetLeafResourceByNodeName(rootpod.Spec.NodeName)
	if err != nil {
		// wait for leaf resource init
		return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
	}

	// skip namespace
	if len(lr.Namespace) > 0 && lr.Namespace != rootpod.Namespace {
		return reconcile.Result{}, nil
	}

	// delete pod in leaf
	if !rootpod.GetDeletionTimestamp().IsZero() {
		if err := r.DeletePodInLeafCluster(ctx, lr, request.NamespacedName, true); err != nil {
			klog.Errorf("delete pod in leaf error[1]: %v,  %s", err, request.NamespacedName)
			return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
		}
		return reconcile.Result{}, nil
	}

	leafPod := &corev1.Pod{}
	err = lr.Client.Get(ctx, request.NamespacedName, leafPod)

	// create pod in leaf
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.CreatePodInLeafCluster(ctx, lr, &rootpod); err != nil {
				klog.Errorf("create pod inleaf error, err: %s", err)
				return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
			} else {
				return reconcile.Result{}, nil
			}
		} else {
			klog.Errorf("get pod in leaf error[3]: %v,  %s", err, request.NamespacedName)
			return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
		}
	}

	// update pod in leaf
	if podutils.ShouldEnqueue(leafPod, &rootpod) {
		if err := r.UpdatePodInLeafCluster(ctx, lr, &rootpod, leafPod); err != nil {
			return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
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

func (r *RootPodReconciler) createSAInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, sa string, ns string) (*corev1.ServiceAccount, error) {
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

func (r *RootPodReconciler) createSATokenInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, saName string, ns string) (*corev1.Secret, error) {
	satokenKey := types.NamespacedName{
		Namespace: ns,
		Name:      saName,
	}
	sa := &corev1.ServiceAccount{}
	err := r.RootClient.Get(ctx, satokenKey, sa)
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

	masterSecret := &corev1.Secret{}
	err = r.RootClient.Get(ctx, secretKey, masterSecret)
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

func (r *RootPodReconciler) createCAInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, ns string) (*corev1.ConfigMap, error) {
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

	ca := &corev1.ConfigMap{}

	rootCAConfigmapKey := types.NamespacedName{
		Namespace: ns,
		Name:      utils.RooTCAConfigMapName,
	}

	err = r.Client.Get(ctx, rootCAConfigmapKey, ca)
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

func (r *RootPodReconciler) convertAuth(ctx context.Context, lr *leafUtils.LeafResource, pod *corev1.Pod) {
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

func (r *RootPodReconciler) CreatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, pod *corev1.Pod) error {
	if err := podutils.PopulateEnvironmentVariables(ctx, pod, r.envResourceManager); err != nil {
		// span.SetStatus(err)
		return err
	}

	clusterNodeInfo := r.GlobalLeafManager.GetClusterNode(pod.Spec.NodeName)
	if clusterNodeInfo == nil {
		return fmt.Errorf("clusternode info is nil , name: %s", pod.Spec.NodeName)
	}

	basicPod := podutils.FitPod(pod, lr.IgnoreLabels, clusterNodeInfo.LeafMode == leafUtils.ALL)
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

func (r *RootPodReconciler) UpdatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootpod *corev1.Pod, leafpod *corev1.Pod) error {
	// TODO: update env
	// TODO： update config secret pv pvc ...
	klog.V(4).Infof("Updating pod %v/%+v", rootpod.Namespace, rootpod.Name)

	if !podutils.IsKosmosPod(leafpod) {
		klog.V(4).Info("Pod is not created by kosmos tree, ignore")
		return nil
	}
	// not used
	podutils.FitLabels(leafpod.ObjectMeta.Labels, lr.IgnoreLabels)
	podCopy := leafpod.DeepCopy()
	// util.GetUpdatedPod update PodCopy container image, annotations, labels.
	// recover toleration, affinity, tripped ignore labels.
	podutils.GetUpdatedPod(podCopy, rootpod, lr.IgnoreLabels)
	if reflect.DeepEqual(leafpod.Spec, podCopy.Spec) &&
		reflect.DeepEqual(leafpod.Annotations, podCopy.Annotations) &&
		reflect.DeepEqual(leafpod.Labels, podCopy.Labels) {
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
	klog.V(4).Infof("Update pod %v/%+v success ", rootpod.Namespace, rootpod.Name)
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

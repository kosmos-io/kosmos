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

	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	RootPodControllerName = "root-pod-controller"
	RootPodRequeueTime    = 10 * time.Second
)

type RootPodReconciler struct {
	client.Client
	LeafClient client.Client
	RootClient client.Client

	NodeName  string
	Namespace string

	IgnoreLabels         []string
	EnableServiceAccount bool

	DynamicLeafClient dynamic.Interface
	DynamicRootClient dynamic.Interface
}

func (r *RootPodReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, request.NamespacedName, &pod); err != nil {
		if errors.IsNotFound(err) {
			leafPod := &corev1.Pod{}
			err := r.LeafClient.Get(ctx, request.NamespacedName, leafPod)
			if err != nil && errors.IsNotFound(err) {
				return reconcile.Result{}, nil
			}
			// delete leaf pod
			if err := r.DeletePodInLeafCluster(ctx, leafPod); err != nil {
				klog.Errorf("delete pod in leaf error[2]: %v,  %s", err, request.NamespacedName)
				return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
			}
			return reconcile.Result{}, nil
		}
		klog.Errorf("get %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
	}

	// belongs to the current node
	if pod.Spec.NodeName != r.NodeName {
		return reconcile.Result{}, nil
	}

	// skip reservedNS
	if pod.Namespace == utils.ReservedNS {
		return reconcile.Result{}, nil
	}

	// skip namespace
	if len(r.Namespace) > 0 && r.Namespace != pod.Namespace {
		return reconcile.Result{}, nil
	}

	// delete
	if !pod.GetDeletionTimestamp().IsZero() {
		if err := r.DeletePodInLeafCluster(ctx, &pod); err != nil {
			klog.Errorf("delete pod in leaf error[1]: %v,  %s", err, request.NamespacedName)
			return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
		}
		return reconcile.Result{}, nil
	}

	leafPod := &corev1.Pod{}
	err := r.LeafClient.Get(ctx, request.NamespacedName, leafPod)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.CreatePodInLeafCluster(ctx, &pod); err != nil {
				return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
			} else {
				return reconcile.Result{}, nil
			}
		} else {
			klog.Errorf("get pod in leaf error[3]: %v,  %s", err, request.NamespacedName)
			return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
		}
	}

	if utils.ShouldEnqueue(leafPod, &pod) {
		if err := r.UpdatePodInLeafCluster(ctx, &pod); err != nil {
			return reconcile.Result{RequeueAfter: RootPodRequeueTime}, nil
		}
	}

	return reconcile.Result{}, nil
}

func (r *RootPodReconciler) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(RootPodControllerName).
		WithOptions(controller.Options{}).
		For(&corev1.Pod{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return true
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return true
			},
		})).
		Complete(r)
}

func (p *RootPodReconciler) createStorageInLeafCluster(ctx context.Context, gvr schema.GroupVersionResource, resourcenames []string, ns string) error {
	for _, rname := range resourcenames {
		_, err := p.DynamicLeafClient.Resource(gvr).Namespace(ns).Get(ctx, rname, metav1.GetOptions{})
		if err == nil {
			continue
		}
		if errors.IsNotFound(err) {
			unstructuredObj, err := p.DynamicRootClient.Resource(gvr).Namespace(ns).Get(ctx, rname, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("find gvr(%v) %v error %v", gvr, rname, err)
			}

			utils.FitUnstructuredObjMeta(unstructuredObj)
			if gvr.Resource == "secrets" {
				secretObj := &corev1.Secret{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, secretObj)
				if err != nil {
					panic(err.Error())
				}
				if secretObj.Type == corev1.SecretTypeServiceAccountToken {
					if err := p.createServiceAccountInLeafCluster(ctx, secretObj); err != nil {
						klog.Error(err)
						return err
					}
				}
			}

			utils.SetUnstructuredObjGlobal(unstructuredObj)

			_, err = p.DynamicLeafClient.Resource(gvr).Namespace(ns).Create(ctx, unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				if errors.IsAlreadyExists(err) {
					continue
				}
				klog.Errorf("Failed to create gvr(%v) %v err: %v", gvr, rname, err)
				return err
			}
			klog.Infof("Create gvr(%v) %v in %v success", gvr, rname, ns)
			continue
		}
		return fmt.Errorf("could not check gvr(%v) %s in external cluster: %v", gvr, rname, err)
	}
	return nil
}

func (p *RootPodReconciler) createSAInLeafCluster(ctx context.Context, sa string, ns string) (*corev1.ServiceAccount, error) {
	saKey := types.NamespacedName{
		Namespace: ns,
		Name:      sa,
	}

	clientSA := &corev1.ServiceAccount{}
	err := p.LeafClient.Get(ctx, saKey, clientSA)
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
	err = p.LeafClient.Create(ctx, newSA)
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("could not create sa %s in member cluster: %v", sa, err)
	}

	return newSA, nil
}

func (p *RootPodReconciler) createSATokenInLeafCluster(ctx context.Context, saName string, ns string) (*corev1.Secret, error) {
	satokenKey := types.NamespacedName{
		Namespace: ns,
		Name:      saName,
	}
	sa := &corev1.ServiceAccount{}
	err := p.RootClient.Get(ctx, satokenKey, sa)
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
	err = p.LeafClient.Get(ctx, csKey, clientSecret)
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
	err = p.RootClient.Get(ctx, secretKey, masterSecret)
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
	err = p.LeafClient.Create(ctx, newSE)

	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("could not create sa %s in member cluster: %v", sa, err)
	}
	return newSE, nil
}

func (p *RootPodReconciler) createCAInLeafCluster(ctx context.Context, ns string) (*corev1.ConfigMap, error) {
	masterCAConfigmapKey := types.NamespacedName{
		Namespace: ns,
		Name:      utils.MasterRooTCAName,
	}

	masterCA := &corev1.ConfigMap{}

	err := p.LeafClient.Get(ctx, masterCAConfigmapKey, masterCA)
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

	err = p.LeafClient.Get(ctx, rootCAConfigmapKey, ca)
	if err != nil {
		return nil, fmt.Errorf("could not find configmap %s in master cluster: %v", ca, err)
	}

	newCA := ca.DeepCopy()
	newCA.Name = utils.MasterRooTCAName
	utils.FitObjectMeta(&newCA.ObjectMeta)

	err = p.LeafClient.Create(ctx, newCA)
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("could not create configmap %s in member cluster: %v", newCA.Name, err)
	}

	return newCA, nil
}

func (p *RootPodReconciler) convertAuth(ctx context.Context, pod *corev1.Pod) {
	if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		falseValue := false
		pod.Spec.AutomountServiceAccountToken = &falseValue

		sa := pod.Spec.ServiceAccountName
		_, err := p.createSAInLeafCluster(ctx, sa, pod.Namespace)
		if err != nil {
			klog.Errorf("[convertAuth] create sa failed, ns: %s, pod: %s", pod.Namespace, pod.Name)
			return
		}

		se, err := p.createSATokenInLeafCluster(ctx, sa, pod.Namespace)
		if err != nil {
			klog.Errorf("[convertAuth] create sa secret failed, ns: %s, pod: %s", pod.Namespace, pod.Name)
			return
		}

		rootCA, err := p.createCAInLeafCluster(ctx, pod.Namespace)
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

func (p *RootPodReconciler) createServiceAccountInLeafCluster(ctx context.Context, secret *corev1.Secret) error {
	if !p.EnableServiceAccount {
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
	sa := &corev1.ServiceAccount{}
	saKey := types.NamespacedName{
		Namespace: ns,
		Name:      accountName,
	}

	err := p.LeafClient.Get(ctx, saKey, sa)
	if err != nil || sa == nil {
		klog.Infof("get serviceAccount [%v] err: [%v]]", sa, err)
		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      accountName,
				Namespace: ns,
			},
		}
		err := p.LeafClient.Create(ctx, sa)
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

	secret.ObjectMeta.Namespace = ns

	err = p.LeafClient.Create(ctx, secret)

	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		klog.Errorf("Failed to create secret %v err: %v", secret.Name, err)
	}

	sa.Secrets = []corev1.ObjectReference{{Name: secret.Name}}

	err = p.LeafClient.Update(ctx, sa)
	if err != nil {
		klog.Infof(
			"update serviceAccount [%v] err: [%v]]",
			sa, err)
		return err
	}
	return nil
}

func (p *RootPodReconciler) CreatePodInLeafCluster(ctx context.Context, pod *corev1.Pod) error {
	if pod.Namespace == utils.ReservedNS {
		return nil
	}
	basicPod := utils.FitPod(pod, p.IgnoreLabels)
	klog.Infof("Creating pod %v/%+v", pod.Namespace, pod.Name)
	ns := &corev1.Namespace{}
	nsKey := types.NamespacedName{
		Name: pod.Namespace,
	}
	if err := p.LeafClient.Get(ctx, nsKey, ns); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.Infof("Namespace %s does not exist for pod %s, creating it", pod.Namespace, pod.Name)
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: pod.Namespace,
			},
		}

		if createErr := p.LeafClient.Create(ctx, ns); createErr != nil && errors.IsAlreadyExists(createErr) {
			klog.Infof("Namespace %s create failed error: %v", pod.Namespace, createErr)
			return err
		}
	}
	secretNames := utils.GetSecrets(pod)
	configMaps := utils.GetConfigmaps(pod)
	pvcs := utils.GetPVCs(pod)
	// nolint:errcheck
	go wait.PollImmediate(500*time.Millisecond, 10*time.Minute, func() (bool, error) {
		klog.Info("Trying to creating base dependent")
		if err := p.createStorageInLeafCluster(ctx, utils.GVR_CONFIGMAP, configMaps, pod.Namespace); err != nil {
			klog.Error(err)
			return false, nil
		}

		klog.Infof("Create configmaps %v of %v/%v success", configMaps, pod.Namespace, pod.Name)
		if err := p.createStorageInLeafCluster(ctx, utils.GVR_PVC, pvcs, pod.Namespace); err != nil {
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

		if err = p.createStorageInLeafCluster(ctx, utils.GVR_SECRET, secretNames, pod.Namespace); err != nil {
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

	err = p.LeafClient.Create(ctx, basicPod)
	if err != nil {
		return fmt.Errorf("could not create pod: %v", err)
	}
	klog.Infof("Create pod %v/%+v success", pod.Namespace, pod.Name)
	return nil
}

func (p *RootPodReconciler) UpdatePodInLeafCluster(ctx context.Context, pod *corev1.Pod) error {
	// TODO: update env
	// TODO： update config secret pv pvc ...
	if pod.Namespace == utils.ReservedNS {
		return nil
	}
	klog.Infof("Updating pod %v/%+v", pod.Namespace, pod.Name)
	currentPod, err := p.GetPodInLeafCluster(ctx, pod.Namespace, pod.Name)
	if err != nil {
		return fmt.Errorf("could not get current pod")
	}
	if !utils.IsKosmosPod(pod) {
		klog.Info("Pod is not created by vk, ignore")
		return nil
	}
	utils.FitLabels(currentPod.ObjectMeta.Labels, p.IgnoreLabels)
	podCopy := currentPod.DeepCopy()
	// util.GetUpdatedPod update PodCopy container image, annotations, labels.
	// recover toleration, affinity, tripped ignore labels.
	utils.GetUpdatedPod(podCopy, pod, p.IgnoreLabels)
	if reflect.DeepEqual(currentPod.Spec, podCopy.Spec) &&
		reflect.DeepEqual(currentPod.Annotations, podCopy.Annotations) &&
		reflect.DeepEqual(currentPod.Labels, podCopy.Labels) {
		return nil
	}

	err = p.LeafClient.Update(ctx, podCopy)
	if err != nil {
		return fmt.Errorf("could not update pod: %v", err)
	}
	klog.Infof("Update pod %v/%+v success ", pod.Namespace, pod.Name)
	return nil
}

func (p *RootPodReconciler) DeletePodInLeafCluster(ctx context.Context, pod *corev1.Pod) error {
	if pod.Namespace == utils.ReservedNS {
		return nil
	}
	klog.Infof("Deleting pod %v/%+v", pod.Namespace, pod.Name)

	if !utils.IsKosmosPod(pod) {
		klog.Info("Pod is not create by vk, ignore")
		return nil
	}

	opts := &metav1.DeleteOptions{
		GracePeriodSeconds: new(int64), // 0
	}
	if pod.DeletionGracePeriodSeconds != nil {
		opts.GracePeriodSeconds = pod.DeletionGracePeriodSeconds
	}

	err := p.LeafClient.Delete(ctx, pod)
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

func (p *RootPodReconciler) GetPodInLeafCluster(ctx context.Context, namespace string, name string) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	err := p.LeafClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, pod)
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

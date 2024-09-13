package util

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

func CreateOrUpdateService(client clientset.Interface, svc *v1.Service) error {
	_, err := client.CoreV1().Services(svc.GetNamespace()).Update(context.TODO(), svc, metav1.UpdateOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		_, err := client.CoreV1().Services(svc.GetNamespace()).Create(context.TODO(), svc, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated svc", "svc", svc.GetName())
	return nil
}

func CreateOrUpdateDeployment(client clientset.Interface, deployment *appsv1.Deployment) error {
	_, err := client.AppsV1().Deployments(deployment.GetNamespace()).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.AppsV1().Deployments(deployment.GetNamespace()).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated deployment", "deployment", deployment.GetName())
	return nil
}

func DeleteDeployment(client clientset.Interface, deployment string, namespace string) error {
	err := client.AppsV1().Deployments(namespace).Delete(context.TODO(), deployment, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).Infof("Deployment %s/%s not found, skip delete", deployment, namespace)
			return nil
		}
		return err
	}
	klog.V(2).Infof("Delete deployment %s/%s success", deployment, namespace)
	return nil
}

func CreateOrUpdateDaemonSet(client clientset.Interface, daemonSet *appsv1.DaemonSet) error {
	_, err := client.AppsV1().DaemonSets(daemonSet.GetNamespace()).Create(context.TODO(), daemonSet, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.AppsV1().DaemonSets(daemonSet.GetNamespace()).Update(context.TODO(), daemonSet, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated daemonSet", "daemonSet", daemonSet.GetName())
	return nil
}

func DeleteDaemonSet(client clientset.Interface, daemonSet string, namespace string) error {
	err := client.AppsV1().DaemonSets(namespace).Delete(context.TODO(), daemonSet, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).Infof("DaemonSet %s/%s not found, skip delete", daemonSet, namespace)
			return nil
		}
		return err
	}
	klog.V(2).Infof("Delete daemonSet %s/%s success", daemonSet, namespace)
	return nil
}

func CreateOrUpdateServiceAccount(client clientset.Interface, serviceAccount *v1.ServiceAccount) error {
	_, err := client.CoreV1().ServiceAccounts(serviceAccount.GetNamespace()).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.CoreV1().ServiceAccounts(serviceAccount.GetNamespace()).Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated serviceAccount", "serviceAccount", serviceAccount.GetName())
	return nil
}

func DeleteServiceAccount(client clientset.Interface, serviceAccount string, namespace string) error {
	err := client.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), serviceAccount, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).Infof("ServiceAccount %s/%s not found, skip delete", serviceAccount, namespace)
			return nil
		}
		return err
	}
	klog.V(2).Infof("Delete serviceAccount %s/%s success", serviceAccount, namespace)
	return nil
}

func CreateOrUpdateConfigMap(client clientset.Interface, configMap *v1.ConfigMap) error {
	_, err := client.CoreV1().ConfigMaps(configMap.GetNamespace()).Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.CoreV1().ConfigMaps(configMap.GetNamespace()).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated configMap", "configMap", configMap.GetName())
	return nil
}

func DeleteConfigmap(client clientset.Interface, cm string, namespace string) error {
	err := client.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), cm, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).Infof("Configmap %s/%s not found, skip delete", cm, namespace)
			return nil
		}
		return err
	}
	klog.V(2).Infof("Delete configmap %s/%s success", cm, namespace)
	return nil
}

func CreateOrUpdateStatefulSet(client clientset.Interface, statefulSet *appsv1.StatefulSet) error {
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

func DeleteStatefulSet(client clientset.Interface, sts string, namespace string) error {
	err := client.AppsV1().StatefulSets(namespace).Delete(context.TODO(), sts, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).Infof("Statefulset %s/%s not found, skip delete", sts, namespace)
			return nil
		}
		return err
	}
	klog.V(2).Infof("Delete statefulset %s/%s success", sts, namespace)
	return nil
}

func CreateOrUpdateClusterSA(client clientset.Interface, serviceAccount *v1.ServiceAccount, namespace string) error {
	_, err := client.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		older, err := client.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), serviceAccount.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		serviceAccount.ResourceVersion = older.ResourceVersion
		_, err = client.CoreV1().ServiceAccounts(namespace).Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(4).InfoS("Successfully created or updated serviceAccount", "serviceAccount", serviceAccount.GetName)
	return nil
}

func CreateOrUpdateClusterRole(client clientset.Interface, clusterrole *rbacv1.ClusterRole) error {
	_, err := client.RbacV1().ClusterRoles().Create(context.TODO(), clusterrole, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		older, err := client.RbacV1().ClusterRoles().Get(context.TODO(), clusterrole.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		clusterrole.ResourceVersion = older.ResourceVersion
		_, err = client.RbacV1().ClusterRoles().Update(context.TODO(), clusterrole, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(4).InfoS("Successfully created or updated clusterrole", "clusterrole", clusterrole.GetName)
	return nil
}

func CreateOrUpdateClusterRoleBinding(client clientset.Interface, clusterroleBinding *rbacv1.ClusterRoleBinding) error {
	_, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterroleBinding, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		older, err := client.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterroleBinding.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		clusterroleBinding.ResourceVersion = older.ResourceVersion
		_, err = client.RbacV1().ClusterRoleBindings().Update(context.TODO(), clusterroleBinding, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(4).InfoS("Successfully created or updated clusterrolebinding", "clusterrolebinding", clusterroleBinding.GetName)
	return nil
}

func CreateObject(dynamicClient dynamic.Interface, namespace string, name string, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	klog.V(2).Infof("Create %s, name: %s, namespace: %s", gvr.String(), name, namespace)
	_, err := dynamicClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Warningf("%s %s already exists", gvr.String(), name)
			return nil
		}
		return err
	}
	return nil
}

func ApplyObject(dynamicClient dynamic.Interface, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	namespace := obj.GetNamespace()
	name := obj.GetName()

	klog.V(2).Infof("Apply %s, name: %s, namespace: %s", gvr.String(), name, namespace)

	resourceClient := dynamicClient.Resource(gvr).Namespace(namespace)

	// Get the existing resource
	existingObj, err := resourceClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			// If not found, create the resource
			_, err = resourceClient.Create(context.TODO(), obj, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			klog.V(2).Infof("Created %s %s in namespace %s", gvr.String(), name, namespace)
			return nil
		}
		return err
	}

	// If found, apply changes using Server-Side Apply
	obj.SetResourceVersion(existingObj.GetResourceVersion())
	_, err = resourceClient.Apply(context.TODO(), name, obj, metav1.ApplyOptions{
		FieldManager: "vc-operator-manager",
		Force:        true,
	})
	if err != nil {
		klog.V(2).Infof("Failed to apply changes to %s %s: %v", gvr.String(), name, err)
		return fmt.Errorf("failed to apply changes to %s %s: %v", gvr.String(), name, err)
	}

	klog.V(2).Infof("Applied changes to %s %s in namespace %s", gvr.String(), name, namespace)
	return nil
}

func DeleteObject(dynamicClient dynamic.Interface, namespace string, name string, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	klog.V(2).Infof("Delete %s, name: %s, namespace: %s", gvr.String(), name, namespace)
	err := dynamicClient.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("%s %s already deleted", gvr.String(), name)
			return nil
		}
		return err
	}
	return nil
}

// DecodeYAML unmarshals a YAML document or multidoc YAML as unstructured
// objects, placing each decoded object into a channel.
// code from https://github.com/kubernetes/client-go/issues/216
func DecodeYAML(data []byte) (<-chan *unstructured.Unstructured, <-chan error) {
	var (
		chanErr        = make(chan error)
		chanObj        = make(chan *unstructured.Unstructured)
		multidocReader = utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	)

	go func() {
		defer close(chanErr)
		defer close(chanObj)

		// Iterate over the data until Read returns io.EOF. Every successful
		// read returns a complete YAML document.
		for {
			buf, err := multidocReader.Read()
			if err != nil {
				if err == io.EOF {
					return
				}
				klog.Warningf("failed to read yaml data")
				chanErr <- errors.Wrap(err, "failed to read yaml data")
				return
			}

			// Do not use this YAML doc if it is unkind.
			var typeMeta runtime.TypeMeta
			if err := yaml.Unmarshal(buf, &typeMeta); err != nil {
				continue
			}
			if typeMeta.Kind == "" {
				continue
			}

			// Define the unstructured object into which the YAML document will be
			// unmarshaled.
			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{},
			}

			// Unmarshal the YAML document into the unstructured object.
			if err := yaml.Unmarshal(buf, &obj.Object); err != nil {
				klog.Warningf("failed to unmarshal yaml data")
				chanErr <- errors.Wrap(err, "failed to unmarshal yaml data")
				return
			}

			// Place the unstructured object into the channel.
			chanObj <- obj
		}
	}()

	return chanObj, chanErr
}

// ForEachObjectInYAMLActionFunc is a function that is executed against each
// object found in a YAML document.
// When a non-empty namespace is provided then the object is assigned the
// namespace prior to any other actions being performed with or to the object.
type ForEachObjectInYAMLActionFunc func(context.Context, dynamic.Interface, *unstructured.Unstructured) error

// ForEachObjectInYAML excutes actionFn for each object in the provided YAML.
// If an error is returned then no further objects are processed.
// The data may be a single YAML document or multidoc YAML.
// When a non-empty namespace is provided then all objects are assigned the
// the namespace prior to any other actions being performed with or to the
// object.
func ForEachObjectInYAML(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	data []byte,
	namespace string,
	actionFn ForEachObjectInYAMLActionFunc) error {
	chanObj, chanErr := DecodeYAML(data)
	for {
		select {
		case obj := <-chanObj:
			if obj == nil {
				return nil
			}
			if namespace != "" {
				obj.SetNamespace(namespace)
			}
			klog.Infof("get object %s/%s", obj.GetNamespace(), obj.GetName())
			if err := actionFn(ctx, dynamicClient, obj); err != nil {
				return err
			}
		case err := <-chanErr:
			if err == nil {
				return nil
			}
			klog.Errorf("DecodeYaml error %v", err)
			return errors.Wrap(err, "received error while decoding yaml")
		}
	}
}

func ReplaceObject(dynamicClient dynamic.Interface, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	namespace := obj.GetNamespace()
	name := obj.GetName()

	klog.V(2).Infof("Replace %s, name: %s, namespace: %s", gvr.String(), name, namespace)

	resourceClient := dynamicClient.Resource(gvr).Namespace(namespace)

	// Get the existing resource
	_, err := resourceClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			// If not found, create the resource
			_, err = resourceClient.Create(context.TODO(), obj, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			klog.V(2).Infof("Created %s %s in namespace %s", gvr.String(), name, namespace)
			return nil
		}
		return err
	}

	// If found, delete the existing resource
	err = resourceClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		klog.V(2).Infof("Failed to delete existing %s %s: %v", gvr.String(), name, err)
		return fmt.Errorf("failed to delete existing %s %s: %v", gvr.String(), name, err)
	}

	klog.V(2).Infof("Deleted existing %s %s in namespace %s", gvr.String(), name, namespace)

	// Create the resource with the new object
	_, err = resourceClient.Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		klog.V(2).Infof("Failed to create %s %s: %v", gvr.String(), name, err)
		return fmt.Errorf("failed to create %s %s: %v", gvr.String(), name, err)
	}

	klog.V(2).Infof("Replaced %s %s in namespace %s", gvr.String(), name, namespace)
	return nil
}

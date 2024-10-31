package pv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func setupController() (*LeafPVController, reconcile.Request) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)

	leafClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	rootClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	rootClientSet := fakeclientset.NewSimpleClientset()

	controller := &LeafPVController{
		LeafClient:    leafClient,
		RootClient:    rootClient,
		RootClientSet: rootClientSet,
		ClusterName:   "test-cluster",
		IsOne2OneMode: false,
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-pv",
		},
	}

	return controller, request
}

func createPV(name string, claimRef *v1.ObjectReference) *v1.PersistentVolume {
	return &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{
				v1.ResourceStorage: resource.MustParse("10Gi"),
			},
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				HostPath: &v1.HostPathVolumeSource{Path: "/tmp/data"},
			},
			ClaimRef: claimRef,
		},
	}
}

func createPVC(name string, namespace string, scName *string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: scName,
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
}

func TestReconcile_LeafPV_NotExist(t *testing.T) {
	controller, request := setupController()

	result, err := controller.Reconcile(context.Background(), request)

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestLeafPVController_Reconcile_CreateRootPV(t *testing.T) {
	controller, request := setupController()

	testPVC := createPVC("my-pvc", "default", nil)
	err := controller.LeafClient.Create(context.Background(), testPVC)
	assert.Nil(t, err)

	ownerRef := v1.ObjectReference{
		Kind:       "PersistentVolumeClaim",
		APIVersion: "v1",
		Name:       "my-pvc",
		Namespace:  "default",
		UID:        testPVC.UID,
	}
	testPV := createPV("test-pv", &ownerRef)
	err = controller.LeafClient.Create(context.Background(), testPV)
	assert.Nil(t, err)

	result, err := controller.Reconcile(context.Background(), request)

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

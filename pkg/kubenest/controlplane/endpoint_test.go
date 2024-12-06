package controlplane

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
)

func TestEnsureKosmosSystemNamespace(t *testing.T) {
	t.Run("Namespace exists", func(t *testing.T) {
		client := fakeclientset.NewSimpleClientset(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: constants.KosmosNs,
			},
		})
		err := EnsureKosmosSystemNamespace(client)
		assert.NoError(t, err, "Namespace already exists but failed")
	})

	t.Run("Namespace not exists and created successfully", func(t *testing.T) {
		client := fakeclientset.NewSimpleClientset()
		err := EnsureKosmosSystemNamespace(client)
		assert.NoError(t, err, "Failed to create namespace")
	})

	t.Run("Error creating namespace", func(t *testing.T) {
		client := fakeclientset.NewSimpleClientset()
		client.PrependReactor("create", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("creation error")
		})
		err := EnsureKosmosSystemNamespace(client)
		assert.Error(t, err, "Expected error when creating namespace")
		assert.EqualError(t, err, "failed to create kosmos-system namespace: creation error", "Error message mismatch")
	})
}

func TestCreateOrUpdateAPIServerExternalService(t *testing.T) {
	t.Run("Successfully create Service", func(t *testing.T) {
		client := fakeclientset.NewSimpleClientset()

		endpoint := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.APIServerExternalService,
				Namespace: constants.KosmosNs,
			},
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: []corev1.EndpointAddress{
						{IP: "192.168.1.2"},
					},
					Ports: []corev1.EndpointPort{
						{Port: 6443},
					},
				},
			},
		}

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: constants.KosmosNs,
			},
		}

		_, err := client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
		assert.NoError(t, err)

		_, err = client.CoreV1().Endpoints(constants.KosmosNs).Create(context.TODO(), endpoint, metav1.CreateOptions{})
		assert.NoError(t, err)

		err = CreateOrUpdateAPIServerExternalService(client)
		assert.NoError(t, err)

		svc, err := client.CoreV1().Services(constants.KosmosNs).Get(context.TODO(), constants.APIServerExternalService, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, svc)
		assert.Equal(t, constants.APIServerExternalService, svc.Name)
		assert.Equal(t, int32(6443), svc.Spec.Ports[0].Port)
	})

	t.Run("Error case - Endpoint not found", func(t *testing.T) {
		client := fakeclientset.NewSimpleClientset()
		err := CreateOrUpdateAPIServerExternalService(client)
		assert.Error(t, err)
		assert.Equal(t, "error when getEndPointPort: endpoints \"api-server-external-service\" not found", err.Error())
	})
}

func TestGetEndPointInfo(t *testing.T) {
	t.Run("Successfully retrieve Endpoint info", func(t *testing.T) {
		client := fakeclientset.NewSimpleClientset(&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.APIServerExternalService,
				Namespace: constants.KosmosNs,
			},
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: []corev1.EndpointAddress{
						{IP: "192.168.1.1"},
					},
					Ports: []corev1.EndpointPort{
						{Port: 6443},
					},
				},
			},
		})
		port, ipFamilies, err := getEndPointInfo(client)
		assert.NoError(t, err)
		assert.Equal(t, int32(6443), port)
		assert.True(t, ipFamilies.IPv4)
		assert.False(t, ipFamilies.IPv6)
	})

	t.Run("No subsets in endpoint", func(t *testing.T) {
		client := fakeclientset.NewSimpleClientset(&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.APIServerExternalService,
				Namespace: constants.KosmosNs,
			},
		})
		_, _, err := getEndPointInfo(client)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "No subsets found in the endpoints")
	})
}

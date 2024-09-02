package util

import (
	"context"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// createKubeConfig creates a kubeConfig from the given config and masterOverride.
func createKubeConfig() (*restclient.Config, error) {
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: "../../../ignore_dir/local.conf"},
		&clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}

	kubeConfig.DisableCompression = true
	kubeConfig.QPS = 40.0
	kubeConfig.Burst = 60

	return kubeConfig, nil
}

func prepare() (clientset.Interface, error) {
	// Prepare kube config.
	kubeConfig, err := createKubeConfig()
	if err != nil {
		return nil, err
	}

	hostKubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %v", err)
	}

	return hostKubeClient, nil
}

func TestCreateOrUpdate(t *testing.T) {
	client, err := prepare()
	if err != nil {
		t.Logf("failed to prepare client: %v", err)
		return
	}

	tests := []struct {
		name  string
		input *v1.Service
		want  bool
	}{
		{
			name: "basic",
			input: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-nodeport-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeNodePort,
					Selector: map[string]string{
						"app": "my-app",
					},
					Ports: []v1.ServicePort{
						{
							Port:     30007, // 服务的端口
							Protocol: v1.ProtocolTCP,
							TargetPort: intstr.IntOrString{
								IntVal: 8080, // Pod 中的目标端口
							},
							NodePort: 30007, // 固定的 NodePort 端口
						},
					},
				},
			},
			want: true,
		},
		{
			name: "same port",
			input: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-nodeport-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeNodePort,
					Selector: map[string]string{
						"app": "my-app",
					},
					Ports: []v1.ServicePort{
						{
							Port:     30007, // 服务的端口
							Protocol: v1.ProtocolTCP,
							TargetPort: intstr.IntOrString{
								IntVal: 8080, // Pod 中的目标端口
							},
							NodePort: 30007, // 固定的 NodePort 端口
						},
					},
				},
			},
			want: true,
		},
		{
			name: "different port",
			input: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-nodeport-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeNodePort,
					Selector: map[string]string{
						"app": "my-app",
					},
					Ports: []v1.ServicePort{
						{
							Port:     30077, // 服务的端口
							Protocol: v1.ProtocolTCP,
							TargetPort: intstr.IntOrString{
								IntVal: 8080, // Pod 中的目标端口
							},
							NodePort: 30077, // 固定的 NodePort 端口
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateOrUpdateService(client, tt.input)
			if err != nil {
				t.Fatalf("CreateOrUpdateService() error = %v", err)
			}
		})
	}
}

func TestCreateSvc(t *testing.T) {
	client, err := prepare()
	if err != nil {
		t.Logf("failed to prepare client: %v", err)
		return
	}

	tests := []struct {
		name   string
		input  *v1.Service
		update *v1.Service
		want   bool
	}{
		{
			name: "ipv4 only",
			input: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-nodeport-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeNodePort,
					Selector: map[string]string{
						"app": "my-app",
					},
					Ports: []v1.ServicePort{
						{
							Port:     30007, // 服务的端口
							Protocol: v1.ProtocolTCP,
							TargetPort: intstr.IntOrString{
								IntVal: 8080, // Pod 中的目标端口
							},
							// NodePort: 30007, // 固定的 NodePort 端口
						},
					},
				},
			},
			update: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-nodeport-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeNodePort,
					Selector: map[string]string{
						"app": "my-app",
					},
					Ports: []v1.ServicePort{
						{
							Port:     30007, // 服务的端口
							Protocol: v1.ProtocolTCP,
							TargetPort: intstr.IntOrString{
								IntVal: 8080, // Pod 中的目标端口
							},
							// NodePort: 30007, // 固定的 NodePort 端口
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateOrUpdateService(client, tt.input)
			if err != nil {
				t.Fatalf("CreateOrUpdateService() error = %v", err)
			}
			svc, err := client.CoreV1().Services(tt.input.Namespace).Get(context.TODO(), tt.input.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("CreateOrUpdateService() error = %v", err)
			}
			nodePort := svc.Spec.Ports[0].NodePort
			tt.update.Spec.Ports[0].NodePort = nodePort
			tt.update.Spec.Ports[0].Port = nodePort
			tt.update.Spec.Ports[0].TargetPort = intstr.IntOrString{
				IntVal: nodePort,
			}
			err = CreateOrUpdateService(client, tt.update)
			if err != nil {
				t.Fatalf("CreateOrUpdateService() error = %v", err)
			}
		})
	}
}

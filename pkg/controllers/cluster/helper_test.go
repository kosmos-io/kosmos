package cluster

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

func TestResolveServiceCIDRs(t *testing.T) {
	t.Run("test resolve service CIDRs", func(t *testing.T) {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "serviceCIDRs-pod",
				Namespace: "kube-system",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "serviceCIDRs-container",
						Image: "kube-apiserver:v1.21.5",
						Command: []string{
							"kube-apiserver",
							"--advertise-address=127.0.0.1",
							"--allow-privileged=true",
							"--alsologtostderr=true",
							"--anonymous-auth=True",
							"--apiserver-count=1",
							"--authorization-mode=Node,RBAC",
							"--bind-address=0.0.0.0",
							"--client-ca-file=/apps/conf/kubernetes/ssl/ca.crt",
							"--enable-admission-plugins=NodeRestriction",
							"--enable-aggregator-routing=False",
							"--enable-bootstrap-token-auth=true",
							"--endpoint-reconciler-type=lease",
							"--etcd-cafile=/apps/conf/kubernetes/ssl/etcd/ca.crt",
							"--etcd-certfile=/apps/conf/kubernetes/ssl/apiserver-etcd-client.crt",
							"--etcd-keyfile=/apps/conf/kubernetes/ssl/apiserver-etcd-client.key",
							"--etcd-servers=https://127.0.0.1:2379",
							"--feature-gates=RotateKubeletServerCertificate=True",
							"--insecure-port=0",
							"--kubelet-client-certificate=/apps/conf/kubernetes/ssl/apiserver-kubelet-client.crt",
							"--kubelet-client-key=/apps/conf/kubernetes/ssl/apiserver-kubelet-client.key",
							"--kubelet-preferred-address-types=InternalDNS,InternalIP,Hostname,ExternalDNS,ExternalIP",
							"--log-file=/home/kube_apiserver.log",
							"--logtostderr=false",
							"--profiling=False",
							"--proxy-client-cert-file=/apps/conf/kubernetes/ssl/front-proxy-client.crt",
							"--proxy-client-key-file=/apps/conf/kubernetes/ssl/front-proxy-client.key",
							"--request-timeout=1m0s",
							"--requestheader-allowed-names=front-proxy-client",
							"--requestheader-client-ca-file=/apps/conf/kubernetes/ssl/front-proxy-ca.crt",
							"--requestheader-extra-headers-prefix=X-Remote-Extra-",
							"--requestheader-group-headers=X-Remote-Group",
							"--requestheader-username-headers=X-Remote-User",
							"--secure-port=6443",
							"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
							"--service-account-key-file=/apps/conf/kubernetes/ssl/sa.pub",
							"--service-account-signing-key-file=/apps/conf/kubernetes/ssl/sa.key",
							"--service-cluster-ip-range=10.233.0.0/18,10.234.0.0/26",
							"--service-node-port-range=30000-32767",
							"--storage-backend=etcd3",
							"--tls-cert-file=/apps/conf/kubernetes/ssl/apiserver.crt",
							"--tls-private-key-file=/apps/conf/kubernetes/ssl/apiserver.key",
						},
						Args: nil,
					},
				},
			},
			Status: v1.PodStatus{
				Phase: "Running",
			},
		}

		serviceCIDRs, err := ResolveServiceCIDRs(pod)
		if err != nil {
			t.Errorf(" ResolveServiceCIDRs = %v, wantErr %v", serviceCIDRs, err)
			return
		}
	})
}

func TestCheckIsEtcd(t *testing.T) {
	t.Run("test check store type is etcd", func(t *testing.T) {
		cluster := clusterlinkv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-name",
				Namespace: "clusterlink-system",
				Annotations: map[string]string{
					DataStoreType: EtcdV3,
				},
			},
		}

		isEtcd := CheckIsEtcd(&cluster)
		if !isEtcd {
			t.Errorf(" CheckIsEtcd = %v, wantErr %v", isEtcd, "store type is"+cluster.Annotations[DataStoreType])
			return
		}
	})
}

func TestGetCalicoClient(t *testing.T) {
	t.Run("test check store type is etcd", func(t *testing.T) {
		cluster := clusterlinkv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-name",
				Namespace: "clusterlink-system",
				Annotations: map[string]string{
					DataStoreType: EtcdV3,
				},
			},
		}

		calicoClient, err := GetCalicoClient(&cluster)
		if err != nil {
			t.Errorf(" GetCalicoClient = %v, wantErr %v", calicoClient, err)
			return
		}
	})
}

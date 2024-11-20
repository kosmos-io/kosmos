package manifest

const (
	CorednsCorefile = `
apiVersion: v1
data:
  Corefile: |
    .:53 {
        errors
        health {
            lameduck 5s
        }
        ready
        kubernetes kosmos.local cluster.local in-addr.arpa ip6.arpa {
            pods insecure
            ttl 30
        }
        hosts /etc/add-hosts/customer-hosts . {
            fallthrough kosmos.local cluster.local in-addr.arpa ip6.arpa
        }
        prometheus :9153
        cache 30
        reload
        loadbalance
    }
kind: ConfigMap
metadata:
  name: coredns
  namespace: {{ .Namespace }}
`

	CorednsCustomerHosts = `
apiVersion: v1
data:
  customer-hosts: |
    #customer-hosts
    #10.10.10.10 myhost1
kind: ConfigMap
metadata:
  name: coredns-customer-hosts
  namespace: {{ .Namespace }}
`

	SchedulerConfigmap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: scheduler-config
  namespace: {{ .Namespace }}
data:
  scheduler-config.yaml: |
    apiVersion: kubescheduler.config.k8s.io/v1beta1
    kind: KubeSchedulerConfiguration
    leaderElection:
      leaderElect: true
      resourceName: kosmos-scheduler
      resourceNamespace: {{ .Namespace }}
    profiles:
      - schedulerName: default-scheduler
        plugins:
          preFilter:
            disabled:
              - name: "VolumeBinding"
            enabled:
              - name: "LeafNodeVolumeBinding"
          filter:
            disabled:
              - name: "VolumeBinding"
              - name: "TaintToleration"
              - name: "LeafNodeDistribution"
            enabled:
              - name: "LeafNodeTaintToleration"
              - name: "LeafNodeVolumeBinding"
          score:
            disabled:
              - name: "VolumeBinding"
          reserve:
            disabled:
              - name: "VolumeBinding"
            enabled:
              - name: "LeafNodeVolumeBinding"
          preBind:
            disabled:
              - name: "VolumeBinding"
            enabled:
              - name: "LeafNodeVolumeBinding"
        pluginConfig:
          - name: LeafNodeVolumeBinding
            args:
              bindTimeoutSeconds: 5
          - name: LeafNodeDistribution
            args:
            # kubeConfigPath: "REPLACE_ME_WITH_KUBE_CONFIG_PATH"
              kubeConfigPath: "/etc/kubernetes/kubeconfig"
`
)

type ConfigmapReplace struct {
	Namespace string
}

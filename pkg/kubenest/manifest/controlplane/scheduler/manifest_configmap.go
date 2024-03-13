package kube_controller

const (
	VirtualClusterSchedulerConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: scheduler-config
  namespace: {{ .Namespace }}
data:
  scheduler-config.yaml: |
    apiVersion: kubescheduler.config.k8s.io/v1
    kind: KubeSchedulerConfiguration
    leaderElection:
      leaderElect: true
      resourceName: {{ .DeploymentName }}
      resourceNamespace:  kube-system
    clientConnection:
      kubeconfig: /etc/virtualcluster/kubeconfig
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
`
)

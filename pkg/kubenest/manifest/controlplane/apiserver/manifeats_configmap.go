package apiserver

const (
	EgressSelectorConfiguration = `
apiVersion: v1
data:
  egress_selector_configuration.yaml: |
    apiVersion: apiserver.k8s.io/v1beta1
    kind: EgressSelectorConfiguration
    egressSelections:
    - name: cluster
      connection:
        proxyProtocol: GRPC
        transport:
          uds:
            udsName: /etc/kubernetes/konnectivity-server/{{ .Namespace }}/{{ .Name }}/konnectivity-server.socket
    - name: master
      connection:
        proxyProtocol: Direct
    - name: etcd
      connection:
        proxyProtocol: Direct
kind: ConfigMap
metadata:
  name: kas-proxy-files
  namespace: {{ .Namespace }}
`
)

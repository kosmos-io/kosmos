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
        proxyProtocol: {{ if eq .AnpMode "uds" }}GRPC{{ else }}HTTPConnect{{ end }}
        transport:
          {{ if eq .AnpMode "uds" }}
          uds:
            udsName: /etc/kubernetes/konnectivity-server/{{ .Namespace }}/{{ .Name }}/konnectivity-server.socket
          {{ else }}
          tcp:
            url: https://{{ .SvcName }}:{{ .ProxyServerPort }}
            tlsConfig:
              caBundle: /etc/virtualcluster/pki/ca.crt
              clientKey: /etc/virtualcluster/pki/proxy-server.key
              clientCert: /etc/virtualcluster/pki/proxy-server.crt
          {{ end }}
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

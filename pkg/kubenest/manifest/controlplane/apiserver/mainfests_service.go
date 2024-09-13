package apiserver

const (
	ApiserverService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    virtualCluster-app: apiserver
    app.kubernetes.io/managed-by: virtual-cluster-controller
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
spec:
  ipFamilies:
  {{- range .IPFamilies }}
  - {{ . }}
  {{- end }}
  ports:
  - name: client
    port: {{ .ServicePort }}
    protocol: TCP
    targetPort: {{ .ServicePort }}
    {{ if .UseAPIServerNodePort }}
    nodePort: {{ .ServicePort }}
    {{ end }}
  selector:
    virtualCluster-app: apiserver
  type: {{ .ServiceType }}
`
	ApiserverAnpService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    virtualCluster-app: apiserver
    app.kubernetes.io/managed-by: virtual-cluster-controller
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
spec:
  ports:
  - name: proxy-server
    port: {{ .ProxyServerPort }}
    protocol: TCP
    targetPort: {{ .ProxyServerPort }}
  selector:
    virtualCluster-app: apiserver
  type: ClusterIP
`
)

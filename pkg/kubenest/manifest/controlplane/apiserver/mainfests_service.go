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
  namespace: "{{ .Namespace }}"
spec:
  ports:
  - name: client
    port: {{ .ServicePort }}
    protocol: TCP
    targetPort: {{ .ServicePort }}
  selector:
    virtualCluster-app: apiserver
  type: {{ .ServiceType }}
`
)

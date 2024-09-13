package virtualcluster

const (
	APIServerExternalService = `
apiVersion: v1
kind: Service
metadata:
  name: api-server-external-service
  namespace: default
spec:
  type: NodePort
  ports:
    - protocol: TCP
      port: {{ .ServicePort }}
      targetPort: {{ .ServicePort }}
      nodePort: 30443
`
)

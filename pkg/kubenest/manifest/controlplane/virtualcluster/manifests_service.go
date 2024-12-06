package virtualcluster

const (
	APIServerExternalService = `
apiVersion: v1
kind: Service
metadata:
  name: api-server-external-service
  namespace: kosmos-system
spec:
  ipFamilies:
    {{- if .IPv4 }} 
    - IPv4
    {{- end }}
    {{- if .IPv6 }} 
    - IPv6
    {{- end }}
  ipFamilyPolicy: PreferDualStack
  type: NodePort
  ports:
    - name: https
      protocol: TCP
      port: {{ .ServicePort }}
      targetPort: {{ .ServicePort }}
      nodePort: 30443
  sessionAffinity: None
`
)

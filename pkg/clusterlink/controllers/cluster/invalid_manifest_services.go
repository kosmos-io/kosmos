package cluster

const (
	InvalidService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    kosmos.io/app: coredns
  name: invalidsvc
  namespace: {{ .Namespace }}
spec:
  clusterIP: 8.8.8.8
  clusterIPs:
  - 8.8.8.8
  ipFamilies:
  - IPv4
  ports:
    - name: dns
      port: 53
      protocol: UDP
      targetPort: 53
  selector:
    invalid/app: null
  sessionAffinity: None
  type: ClusterIP
`
)

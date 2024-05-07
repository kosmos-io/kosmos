package etcd

const (
	// EtcdClientService is etcd client service manifest
	EtcdClientService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    virtualCluster-app: etcd
    app.kubernetes.io/managed-by: virtual-cluster-controller
  name: {{ .ServiceName }}
  namespace: "{{ .Namespace }}"
spec:
  ports:
  - name: client
    port: {{ .EtcdListenClientPort }}
    protocol: TCP
    targetPort: {{ .EtcdListenClientPort }}
  selector:
    virtualCluster-app: etcd
  type: ClusterIP
 `

	// EtcdPeerService is etcd peer Service manifest
	EtcdPeerService = `
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     virtualCluster-app: etcd
     app.kubernetes.io/managed-by: virtual-cluster-controller
   name: {{ .ServiceName }}
   namespace: "{{ .Namespace }}"
 spec:
   clusterIP: None
   ports:
   - name: client
     port: {{ .EtcdListenClientPort }}
     protocol: TCP
     targetPort: {{ .EtcdListenClientPort }}
   - name: server
     port: {{ .EtcdListenPeerPort }}
     protocol: TCP
     targetPort: {{ .EtcdListenPeerPort }}
   selector:
     virtualCluster-app: etcd
   type: ClusterIP
  `
)

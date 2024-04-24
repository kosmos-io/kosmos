package apiserver

const (
	ApiserverDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    virtualCluster-app: apiserver
    app.kubernetes.io/managed-by: virtual-cluster-controller
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      virtualCluster-app: apiserver
  template:
    metadata:
      labels:
        virtualCluster-app: apiserver
    spec:
      automountServiceAccountToken: false
      hostNetwork: true
      containers:
      - name: kube-apiserver
        image:  {{ .ImageRepository }}/kube-apiserver:{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
        - kube-apiserver
        - --allow-privileged=true
        - --authorization-mode=Node,RBAC
        - --client-ca-file=/etc/virtualcluster/pki/ca.crt
        - --enable-admission-plugins=NodeRestriction
        - --enable-bootstrap-token-auth=true
        - --etcd-cafile=/etc/etcd/pki/etcd-ca.crt
        - --etcd-certfile=/etc/etcd/pki/etcd-client.crt
        - --etcd-keyfile=/etc/etcd/pki/etcd-client.key
        #- --etcd-servers=https://{{ .EtcdClientService }}.{{ .Namespace }}.svc.cluster.local:{{ .EtcdListenClientPort }}
        - --etcd-servers=https://{{ .EtcdClientService }}:{{ .EtcdListenClientPort }}
        - --bind-address=0.0.0.0
        - --kubelet-client-certificate=/etc/virtualcluster/pki/virtualCluster.crt
        - --kubelet-client-key=/etc/virtualcluster/pki/virtualCluster.key
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
        - --secure-port={{ .ClusterPort }}
        - --service-account-issuer=https://kubernetes.default.svc.cluster.local
        - --service-account-key-file=/etc/virtualcluster/pki/virtualCluster.key
        - --service-account-signing-key-file=/etc/virtualcluster/pki/virtualCluster.key
        - --service-cluster-ip-range={{ .ServiceSubnet }}
        - --proxy-client-cert-file=/etc/virtualcluster/pki/front-proxy-client.crt
        - --proxy-client-key-file=/etc/virtualcluster/pki/front-proxy-client.key
        - --requestheader-allowed-names=front-proxy-client
        - --requestheader-client-ca-file=/etc/virtualcluster/pki/front-proxy-ca.crt
        - --requestheader-extra-headers-prefix=X-Remote-Extra-
        - --requestheader-group-headers=X-Remote-Group
        - --requestheader-username-headers=X-Remote-User
        - --tls-cert-file=/etc/virtualcluster/pki/apiserver.crt
        - --tls-private-key-file=/etc/virtualcluster/pki/apiserver.key
        - --tls-min-version=VersionTLS13
        - --max-requests-inflight=1500
        - --max-mutating-requests-inflight=500
        - --v=4
        livenessProbe:
          failureThreshold: 8
          httpGet:
            path: /livez
            port: {{ .ClusterPort }}
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 15
        readinessProbe:
          failureThreshold: 3
          httpGet:
            path: /readyz
            port: {{ .ClusterPort }}
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 15
        affinity:
          podAntiAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
              matchExpressions:
              - key: virtualCluster-app
                operator: In
                values:
                - apiserver
              topologyKey: kubernetes.io/hostname
        ports:
        - containerPort: {{ .ClusterPort }}
          name: http
          protocol: TCP
        volumeMounts:
        - mountPath: /etc/virtualcluster/pki
          name: apiserver-cert
          readOnly: true
        - mountPath: /etc/etcd/pki
          name: etcd-cert
          readOnly: true
      priorityClassName: system-node-critical
      volumes:
      - name: apiserver-cert
        secret:
          secretName: {{ .VirtualClusterCertsSecret }}
      - name: etcd-cert
        secret:
          secretName: {{ .EtcdCertsSecret }}
`
)

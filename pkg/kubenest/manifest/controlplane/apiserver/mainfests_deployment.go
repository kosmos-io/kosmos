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
      dnsPolicy: ClusterFirstWithHostNet
      tolerations:
      - key: "node-role.kubernetes.io/control-plane"
        operator: "Exists"
        effect: "NoSchedule"
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/control-plane
                    operator: Exists
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: virtualCluster-app
                  operator: In
                  values:
                  - apiserver
              topologyKey: kubernetes.io/hostname
      containers:
      - name: kube-apiserver
        image:  {{ .ImageRepository }}/kube-apiserver:{{ .Version }}
        imagePullPolicy: IfNotPresent
        env:
          - name: PODIP
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: status.podIP
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
        - --advertise-address=$(PODIP)
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
	ApiserverAnpDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    virtualCluster-app: apiserver
    virtualCluster-anp: apiserver-anp
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
        virtualCluster-anp: apiserver-anp
    spec:
      automountServiceAccountToken: false
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      tolerations:
      - key: "node-role.kubernetes.io/control-plane"
        operator: "Exists"
        effect: "NoSchedule"
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/control-plane
                    operator: Exists
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: virtualCluster-app
                  operator: In
                  values:
                  - apiserver
              topologyKey: kubernetes.io/hostname
      containers:
      - name: kube-apiserver
        image:  {{ .ImageRepository }}/kube-apiserver:{{ .Version }}
        imagePullPolicy: IfNotPresent
        env:
          - name: PODIP
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: status.podIP
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
        - --advertise-address=$(PODIP)
        - --egress-selector-config-file=/etc/kubernetes/konnectivity-server-config/{{ .Namespace }}/{{ .Name }}/egress_selector_configuration.yaml
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
        - mountPath: /etc/kubernetes/konnectivity-server/{{ .Namespace }}/{{ .Name }}
          readOnly: false
          name: konnectivity-uds
        - name: kas-proxy
          mountPath: /etc/kubernetes/konnectivity-server-config/{{ .Namespace }}/{{ .Name }}/egress_selector_configuration.yaml
          subPath: egress_selector_configuration.yaml
      - name: konnectivity-server-container
        image: {{ .ImageRepository }}/kas-network-proxy-server:{{ .Version }}
        resources:
          requests:
            cpu: 1m
        securityContext:
          allowPrivilegeEscalation: false
          runAsUser: 0
        command: [ "/proxy-server"]
        args: [
          "--log-file=/var/log/{{ .Namespace }}/{{ .Name }}/konnectivity-server.log",
          "--logtostderr=true",
          "--log-file-max-size=0",
          "--cluster-cert=/etc/virtualcluster/pki/apiserver.crt",
          "--cluster-key=/etc/virtualcluster/pki/apiserver.key",
          {{ if eq .AnpMode "uds" }}
          "--server-port=0",
          "--mode=grpc",
          "--uds-name=/etc/kubernetes/konnectivity-server/{{ .Namespace }}/{{ .Name }}/konnectivity-server.socket",
          "--delete-existing-uds-file",
          {{ else }}
          "--server-port={{ .ServerPort }}",
          "--mode=http-connect",
          "--server-cert=/etc/virtualcluster/pki/proxy-server.crt",
          "--server-ca-cert=/etc/virtualcluster/pki/ca.crt",
          "--server-key=/etc/virtualcluster/pki/proxy-server.key",
          {{ end }}
          "--agent-port={{ .AgentPort }}",
          "--health-port={{ .HealthPort }}",
          "--admin-port={{ .AdminPort }}",
          "--keepalive-time=1h",
          "--agent-namespace=kube-system",
          "--agent-service-account=konnectivity-agent",
          "--kubeconfig=/etc/apiserver/kubeconfig",
          "--authentication-audience=system:konnectivity-server",
          ]
        livenessProbe:
          httpGet:
            scheme: HTTP
            host: 127.0.0.1
            port: {{ .HealthPort }}
            path: /healthz
          initialDelaySeconds: 10
          timeoutSeconds: 60
        ports:
        - name: serverport
          containerPort: {{ .ServerPort }}
          hostPort: {{ .ServerPort }}
        - name: agentport
          containerPort: {{ .AgentPort }}
          hostPort: {{ .AgentPort }}
        - name: healthport
          containerPort: {{ .HealthPort }}
          hostPort: {{ .HealthPort }}
        - name: adminport
          containerPort: {{ .AdminPort }}
          hostPort: {{ .AdminPort }}
        volumeMounts:
        - mountPath: /etc/virtualcluster/pki
          name: apiserver-cert
          readOnly: true
        - name: varlogkonnectivityserver
          mountPath: /var/log/{{ .Namespace }}/{{ .Name }}
          readOnly: false
        - name: konnectivity-home
          mountPath: /etc/kubernetes/konnectivity-server/{{ .Namespace }}/{{ .Name }}
        - mountPath: /etc/apiserver/kubeconfig
          name: kubeconfig
          subPath: kubeconfig
      priorityClassName: system-node-critical
      volumes:
      - name: kubeconfig
        secret:
          defaultMode: 420
          secretName: {{ .KubeconfigSecret }}
      - name: varlogkonnectivityserver
        hostPath:
          path: /var/log/{{ .Namespace }}/{{ .Name }}
          type: DirectoryOrCreate
      - name: konnectivity-home
        hostPath:
          path: /etc/kubernetes/konnectivity-server/{{ .Namespace }}/{{ .Name }}
          type: DirectoryOrCreate
      - name: apiserver-cert
        secret:
          secretName: {{ .VirtualClusterCertsSecret }}
      - name: etcd-cert
        secret:
          secretName: {{ .EtcdCertsSecret }}
      - name: konnectivity-uds
        hostPath:
          path: /etc/kubernetes/konnectivity-server/{{ .Namespace }}/{{ .Name }}
          type: DirectoryOrCreate
      - name: kas-proxy
        configMap:
          name: kas-proxy-files
`
	AnpAgentManifest = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:konnectivity-server
  labels:
    kubernetes.io/cluster-service: "true"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: system:konnectivity-server
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: konnectivity-agent
  namespace: kube-system
  labels:
    kubernetes.io/cluster-service: "true"
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    k8s-app: konnectivity-agent
  namespace: kube-system
  name: konnectivity-agent
spec:
  selector:
    matchLabels:
      k8s-app: konnectivity-agent
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        k8s-app: konnectivity-agent
    spec:
      priorityClassName: system-cluster-critical
      tolerations:
        - key: "CriticalAddonsOnly"
          operator: "Exists"
        - operator: "Exists"
          effect: "NoExecute"
      nodeSelector:
        kubernetes.io/os: linux
      dnsPolicy: ClusterFirstWithHostNet
      containers:
        - name: konnectivity-agent-container
          image: {{ .ImageRepository }}/kas-network-proxy-agent:{{ .Version }}
          resources:
            requests:
              cpu: 50m
            limits:
              memory: 30Mi
          command: [ "/proxy-agent"]
          args: [
            "--logtostderr=true",
            "--ca-cert=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
            "--proxy-server-host=konnectivity-server.kube-system.svc.cluster.local",
            "--proxy-server-port={{ .AgentPort }}",
            "--sync-interval=5s",
            "--sync-interval-cap=30s",
            "--probe-interval=5s",
            "--service-account-token-path=/var/run/secrets/tokens/konnectivity-agent-token",
            "--agent-identifiers=ipv4=$(HOST_IP)",
            {{ if ne .AnpMode "uds" }}
            "--agent-cert=/etc/virtualcluster/pki/apiserver.crt",
            "--agent-key=/etc/virtualcluster/pki/apiserver.key",
            {{ end }}
          ]
          env:
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: HOST_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.hostIP
          livenessProbe:
            httpGet:
              scheme: HTTP
              port: 8093
              path: /healthz
            initialDelaySeconds: 15
            timeoutSeconds: 15
          readinessProbe:
            httpGet:
              scheme: HTTP
              port: 8093
              path: /readyz
            initialDelaySeconds: 15
            timeoutSeconds: 15
          volumeMounts:
            - name: agent-cert
              mountPath: /etc/virtualcluster/pki
            - mountPath: /var/run/secrets/tokens
              name: konnectivity-agent-token
      serviceAccountName: konnectivity-agent
      volumes:
        - name: agent-cert
          secret:
            secretName: {{ .AgentCertName }}
        - name: konnectivity-agent-token
          projected:
            sources:
              - serviceAccountToken:
                  path: konnectivity-agent-token
                  audience: system:konnectivity-server
---
apiVersion: v1
kind: Endpoints
metadata:
  name: konnectivity-server
  namespace: kube-system
subsets:
  - addresses:
      {{- range .ProxyServerHost }}
      - ip: {{ . }}
      {{- end }}
    ports:
      - port: {{ .AgentPort }}
        name: proxy-server
---
apiVersion: v1
kind: Service
metadata:
  name: konnectivity-server
  namespace: kube-system
spec:
  ports:
    - port: {{ .AgentPort }}
      name: proxy-server
      targetPort: {{ .AgentPort }}
`
)

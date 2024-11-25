package manifest

const (
	ClusterlinkNetworkManagerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: clusterlink-network-manager
  namespace: {{ .Namespace }}
  labels:
    app: clusterlink-network-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: clusterlink-network-manager
  template:
    metadata:
      labels:
        app: clusterlink-network-manager
    spec:
      serviceAccountName: clusterlink-network-manager
      containers:
        - name: manager
          image: {{ .ImageRepository }}/clusterlink-network-manager:{{ .Version }}
          imagePullPolicy: IfNotPresent
          command:
            - clusterlink-network-manager
            - --v=4
          resources:
            limits:
              memory: 500Mi
              cpu: 500m
            requests:
              cpu: 500m
              memory: 500Mi
`

	ClusterlinkOperatorDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: clusterlink-operator
  namespace: {{ .Namespace }}
  labels:
    app: operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: operator
  template:
    metadata:
      labels:
        app: operator
    spec:
      serviceAccountName: clusterlink-operator
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app
                    operator: In
                    values:
                      - operator
              namespaces:
                - {{ .Namespace }}
              topologyKey: kubernetes.io/hostname
      containers:
      - name: operator
        image: {{ .ImageRepository }}/clusterlink-operator:{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
          - clusterlink-operator
          - --controlpanelconfig=/etc/clusterlink-operator/kubeconfig
        resources:
          limits:
            memory: 200Mi
            cpu: 250m
          requests:
            cpu: 100m
            memory: 200Mi
        env:
        - name: VERSION
          value: {{ .Version }}
        - name: USE_PROXY
          value: "{{ .UseProxy }}"
        volumeMounts:
          - mountPath: /etc/clusterlink-operator
            name: proxy-config
            readOnly: true
      volumes:
        - name: proxy-config
          secret:
            secretName: controlpanel-config
`

	ClusterTreeClusterManagerDeployment = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: clustertree-cluster-manager
  namespace: {{ .Namespace }}
  labels:
    app: clustertree-cluster-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: clustertree-cluster-manager
  template:
    metadata:
      labels:
        app: clustertree-cluster-manager
    spec:
      serviceAccountName: clustertree
      containers:
        - name: manager
          image: {{ .ImageRepository }}/clustertree-cluster-manager:{{ .Version }}
          imagePullPolicy: IfNotPresent
          env:
            - name: APISERVER_CERT_LOCATION
              value: /etc/cluster-tree/cert/cert.pem
            - name: APISERVER_KEY_LOCATION
              value: /etc/cluster-tree/cert/key.pem
            - name: LEAF_NODE_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          volumeMounts:
            - name: credentials
              mountPath: "/etc/cluster-tree/cert"
              readOnly: true
          command:
            - clustertree-cluster-manager
            - --multi-cluster-service=true
            - --v=4
      volumes:
        - name: credentials
          secret:
            secretName: clustertree-cluster-manager
`

	CorednsDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    kosmos.io/app: coredns
  name: coredns
  namespace: {{ .Namespace }}
spec:
  progressDeadlineSeconds: 600
  replicas: 2
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      kosmos.io/app: coredns
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        kosmos.io/app: coredns
    spec:
      containers:
        - args:
            - -conf
            - /etc/coredns/Corefile
          image: {{ .ImageRepository }}/coredns:latest
          imagePullPolicy: IfNotPresent
          livenessProbe:
            failureThreshold: 5
            httpGet:
              path: /health
              port: 8080
              scheme: HTTP
            initialDelaySeconds: 60
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 5
          name: coredns
          ports:
            - containerPort: 53
              name: dns
              protocol: UDP
            - containerPort: 53
              name: dns-tcp
              protocol: TCP
            - containerPort: 9153
              name: metrics
              protocol: TCP
          readinessProbe:
            failureThreshold: 3
            httpGet:
              path: /ready
              port: 8181
              scheme: HTTP
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 1
          resources:
            limits:
              cpu: 2000m
              memory: 2560Mi
            requests:
              cpu: 1000m
              memory: 1280Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              add:
                - NET_BIND_SERVICE
              drop:
                - all
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /etc/coredns
              name: config-volume
              readOnly: true
            - mountPath: /etc/add-hosts
              name: customer-hosts
              readOnly: true
      dnsPolicy: Default
      priorityClassName: system-cluster-critical
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      serviceAccount: coredns
      serviceAccountName: coredns
      terminationGracePeriodSeconds: 30
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  kosmos.io/app: coredns
              topologyKey: kubernetes.io/hostname
      volumes:
        - configMap:
            defaultMode: 420
            items:
              - key: Corefile
                path: Corefile
            name: coredns
          name: config-volume
        - configMap:
            defaultMode: 420
            items:
              - key: customer-hosts
                path: customer-hosts
            name: coredns-customer-hosts
          name: customer-hosts
`

	SchedulerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kosmos-scheduler
  namespace: {{ .Namespace }}
  labels:
    app: scheduler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: scheduler
  template:
    metadata:
      labels:
        app: scheduler
    spec:
      volumes:
        - name: scheduler-config
          configMap:
            name: scheduler-config
            defaultMode: 420
        - name: kubeconfig-path
          configMap:
            name: host-kubeconfig
            defaultMode: 420
      containers:
        - name: kosmos-scheduler
          image: {{ .ImageRepository }}/scheduler:{{ .Version }}
          imagePullPolicy: IfNotPresent
          command:
            - scheduler
            - --config=/etc/kubernetes/kube-scheduler/scheduler-config.yaml
          resources:
            requests:
              cpu: 200m
          volumeMounts:
            - name: scheduler-config
              readOnly: true
              mountPath: /etc/kubernetes/kube-scheduler
            - name: kubeconfig-path
              readOnly: true
              mountPath: /etc/kubernetes
          livenessProbe:
            httpGet:
              path: /healthz
              port: 10259
              scheme: HTTPS
            initialDelaySeconds: 15
            periodSeconds: 10
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /healthz
              port: 10259
              scheme: HTTPS
      restartPolicy: Always
      dnsPolicy: ClusterFirst
      serviceAccountName: kosmos-scheduler
`
)

type DeploymentReplace struct {
	Namespace       string
	ImageRepository string
	Version         string

	UseProxy string
}

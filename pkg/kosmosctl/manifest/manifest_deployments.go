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
          image: {{ .ImageRepository }}/clusterlink-network-manager:v{{ .Version }}
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

	KosmosOperatorDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kosmos-operator
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
      serviceAccountName: kosmos-operator
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
        image: {{ .ImageRepository }}/kosmos-operator:v{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
          - kosmos-operator
          - --controlpanelconfig=/etc/kosmos-operator/kubeconfig
        resources:
          limits:
            memory: 200Mi
            cpu: 250m
          requests:
            cpu: 100m
            memory: 200Mi
        env:
        - name: VERSION
          value: v{{ .Version }}
        - name: USE_PROXY
          value: "{{ .UseProxy }}"
        volumeMounts:
          - mountPath: /etc/kosmos-operator
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
          image: {{ .ImageRepository }}/clustertree-cluster-manager:v{{ .Version }}
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
)

type DeploymentReplace struct {
	Namespace       string
	ImageRepository string
	Version         string

	UseProxy string
}

package proxy

const clusterlinkProxyService = `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  selector:
    app: clusterlink-proxy
  ports:
    - protocol: TCP
      port: 443
      targetPort: 443
  type: ClusterIP
`

type ServiceReplace struct {
	Namespace string
	Name      string
}

const clusterlinkProxyDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app: clusterlink-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: clusterlink-proxy
  template:
    metadata:
      labels:
        app: clusterlink-proxy
    spec:
      containers:
        - name: clusterlink-proxy
          image: {{ .ImageRepository }}/clusterlink-proxy:{{ .Version }}
          imagePullPolicy: IfNotPresent
          command:
            - clusterlink-proxy
            - --kubeconfig=/etc/clusterlink/kubeconfig
            - --authentication-kubeconfig=/etc/clusterlink/kubeconfig
            - --authorization-kubeconfig=/etc/clusterlink/kubeconfig
          resources:
            limits:
              cpu: 500m
              memory: 500Mi
            requests:
              cpu: 500m
              memory: 500Mi
          readinessProbe:
            exec:
              command:
              - cat
              - /proc/1/cmdline
            failureThreshold: 30
            initialDelaySeconds: 3
            periodSeconds: 10
            timeoutSeconds: 5
          livenessProbe:
            failureThreshold: 30
            exec:
              command:
                - cat
                - /proc/1/cmdline
            initialDelaySeconds: 3
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 3
          volumeMounts:
            - mountPath: /etc/clusterlink
              name: proxy-config
              readOnly: true
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app
                    operator: In
                    values:
                      - clusterlink-proxy
            namespaces:
              - {{ .Namespace }}
            topologyKey: kubernetes.io/hostname
      volumes:
        - name: proxy-config
          secret:
            defaultMode: 420
            secretName: {{ .ControlPanelSecretName }}
`

type DeploymentReplace struct {
	Namespace              string
	Name                   string
	ControlPanelSecretName string
	ImageRepository        string
	Version                string
}

package kubecontroller

const (
	// KubeControllerManagerDeployment is KubeControllerManage deployment manifest
	KubeControllerManagerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    virtualCluster-app: kube-controller-manager
    app.kubernetes.io/managed-by: virtual-cluster-controller
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      virtualCluster-app: kube-controller-manager
  template:
    metadata:
      labels:
        virtualCluster-app: kube-controller-manager
    spec:
      automountServiceAccountToken: false
      priorityClassName: system-node-critical
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
                  - kube-controller-manager
              topologyKey: kubernetes.io/hostname
      containers:
      - name: kube-controller-manager
        image:  {{ .ImageRepository }}/kube-controller-manager:{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
        - kube-controller-manager
        - --allocate-node-cidrs=true
        - --kubeconfig=/etc/virtualcluster/kubeconfig
        - --authentication-kubeconfig=/etc/virtualcluster/kubeconfig
        - --authorization-kubeconfig=/etc/virtualcluster/kubeconfig
        - --bind-address=0.0.0.0
        - --client-ca-file=/etc/virtualcluster/pki/ca.crt
        - --cluster-cidr=10.244.0.0/16
        - --cluster-name=virtualcluster
        - --cluster-signing-cert-file=/etc/virtualcluster/pki/ca.crt
        - --cluster-signing-key-file=/etc/virtualcluster/pki/ca.key
        - --controllers=*,namespace,garbagecollector,serviceaccount-token,ttl-after-finished,bootstrapsigner,csrapproving,csrcleaner,csrsigning,clusterrole-aggregation
        - --leader-elect=true
        - --node-cidr-mask-size=24
        - --root-ca-file=/etc/virtualcluster/pki/ca.crt
        - --service-account-private-key-file=/etc/virtualcluster/pki/virtualCluster.key
        - --service-cluster-ip-range={{ .ServiceSubnet }}
        - --use-service-account-credentials=true
        - --v=4
        livenessProbe:
          failureThreshold: 8
          httpGet:
            path: /healthz
            port: 10257
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 15
        volumeMounts:
        - name: virtualcluster-certs
          mountPath: /etc/virtualcluster/pki
          readOnly: true
        - name: kubeconfig
          mountPath: /etc/virtualcluster/kubeconfig
          subPath: kubeconfig
      volumes:
        - name: virtualcluster-certs
          secret:
            secretName: {{ .VirtualClusterCertsSecret }}
        - name: kubeconfig
          secret:
            secretName: {{ .KubeconfigSecret }}
`
)

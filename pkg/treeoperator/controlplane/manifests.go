package controlplane

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
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
              matchExpressions:
              - key: virtualCluster-app
                operator: In
                values: ["kube-controller-manager"]
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
        - --service-cluster-ip-range=10.96.0.0/12
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

	VirtualClusterSchedulerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    virtualCluster-app: scheduler
    app.kubernetes.io/managed-by: virtual-cluster-controller
spec:
  replicas: {{ .Replicas }} 
  selector:
    matchLabels:    
      virtualCluster-app: scheduler
  template:
    metadata:
      labels:
        virtualCluster-app:  scheduler
    spec:
      automountServiceAccountToken: false
      tolerations:    
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:     
      - name: scheduler
        image: {{ .ImageRepository }}/scheduler:{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:        
        - scheduler     
        - --config=/etc/kubernetes/kube-scheduler/scheduler-config.yaml
        - --authentication-kubeconfig=/etc/virtualcluster/kubeconfig
        - --authorization-kubeconfig=/etc/virtualcluster/kubeconfig
        - --v=4
        livenessProbe:  
          httpGet:        
            path: /healthz  
            port: 10259     
            scheme: HTTPS    
          failureThreshold: 3
          initialDelaySeconds: 15
          periodSeconds: 15
          timeoutSeconds: 5
        volumeMounts:   
        - name: kubeconfig
          subPath: kubeconfig
          mountPath: /etc/virtualcluster/kubeconfig
        - name: scheduler-config
          readOnly: true  
          mountPath: /etc/kubernetes/kube-scheduler
      volumes:
        - name: kubeconfig
          secret:         
            secretName: {{ .KubeconfigSecret }}
        - name: scheduler-config
          configMap:
            defaultMode: 420
            name: scheduler-config
`

	VirtualClusterSchedulerConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: scheduler-config
  namespace: {{ .Namespace }}
data:
  scheduler-config.yaml: |
    apiVersion: kubescheduler.config.k8s.io/v1
    kind: KubeSchedulerConfiguration
    leaderElection:
      leaderElect: true
      resourceName: {{ .DeploymentName }}
      resourceNamespace:  kube-system
    clientConnection:
      kubeconfig: /etc/virtualcluster/kubeconfig
    profiles:
      - schedulerName: default-scheduler
        plugins:
          preFilter:
            disabled:
              - name: "VolumeBinding"
            enabled:
              - name: "LeafNodeVolumeBinding"
          filter:
            disabled:
              - name: "VolumeBinding"
              - name: "TaintToleration"
            enabled:
              - name: "LeafNodeTaintToleration"
              - name: "LeafNodeVolumeBinding"
          score:
            disabled:
              - name: "VolumeBinding"
          reserve:
            disabled:
              - name: "VolumeBinding"
            enabled:
              - name: "LeafNodeVolumeBinding"
          preBind:
            disabled:
              - name: "VolumeBinding"
            enabled:
              - name: "LeafNodeVolumeBinding"
        pluginConfig:
          - name: LeafNodeVolumeBinding
            args:
              bindTimeoutSeconds: 5
`
	VirtualClusterSchedulerOldConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: scheduler-config
  namespace: {{ .Namespace }}
data:
  scheduler-config.yaml: |
    apiVersion: kubescheduler.config.k8s.io/v1beta1
    kind: KubeSchedulerConfiguration
    leaderElection:
      leaderElect: true
      resourceName: {{ .DeploymentName }}
      resourceNamespace:  kube-system
    clientConnection:
      kubeconfig: /etc/virtualcluster/kubeconfig
    profiles:
      - schedulerName: default-scheduler
        plugins:
          preFilter:
            disabled:
              - name: "VolumeBinding"
            enabled:
              - name: "KnodeVolumeBinding"
          filter:
            disabled:
              - name: "VolumeBinding"
              - name: "TaintToleration"
            enabled:
              - name: "KNodeTaintToleration"
              - name: "KnodeVolumeBinding"
          score:
            disabled:
              - name: "VolumeBinding"
          reserve:
            disabled:
              - name: "VolumeBinding"
            enabled:
              - name: "KnodeVolumeBinding"
          preBind:
            disabled:
              - name: "VolumeBinding"
            enabled:
              - name: "KnodeVolumeBinding"
        pluginConfig:
          - name: KnodeVolumeBinding
            args:
              bindTimeoutSeconds: 5
`
	/*	SchedulerDeployment = `
		apiVersion: apps/v1
		kind: Deployment
		metadata:
		  name: {{ .DeploymentName }}
		  namespace: {{ .Namespace }}
		  labels:
		    virtualCluster-app: scheduler
		    app.kubernetes.io/managed-by: virtual-cluster-controller
		spec:
		  replicas: {{ .Replicas }}
		  selector:
		    matchLabels:
		      virtualCluster-app: scheduler
		  template:
		    metadata:
		      labels:
		        virtualCluster-app: scheduler
		    spec:
		      automountServiceAccountToken: false
		      containers:
		      - name: scheduler
		        image: {{ .Image }}
		        imagePullPolicy: IfNotPresent
		        command:
		        - /bin/karmada-scheduler
		        - --kubeconfig=/etc/virtualcluster/kubeconfig
		        - --bind-address=0.0.0.0
		        - --secure-port=10351
		        - --enable-scheduler-estimator=true
		        - --leader-elect-resource-namespace={{ .SystemNamespace }}
		        - --v=4
		        livenessProbe:
		          httpGet:
		            path: /healthz
		            port: 10351
		            scheme: HTTP
		          failureThreshold: 3
		          initialDelaySeconds: 15
		          periodSeconds: 15
		          timeoutSeconds: 5
		        volumeMounts:
		        - name: kubeconfig
		          subPath: kubeconfig
		          mountPath: /etc/virtualcluster/kubeconfig
		      volumes:
		        - name: kubeconfig
		          secret:
		            secretName: {{ .KubeconfigSecret }}
		`*/
)

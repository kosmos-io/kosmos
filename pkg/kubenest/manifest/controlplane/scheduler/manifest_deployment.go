package kube_controller

const (
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
)

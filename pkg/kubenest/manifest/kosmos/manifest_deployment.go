package kosmos

const (
	ClusterTreeClusterManagerDeployment = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}-clustertree-cluster-manager
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
      containers:
        - name: manager
          image: {{ .ImageRepository }}/clustertree-cluster-manager:{{ .Version }}
          imagePullPolicy: IfNotPresent
          env:
            - name: APISERVER_CERT_LOCATION
              value: {{ .FilePath }}/cert.pem
            - name: APISERVER_KEY_LOCATION
              value: {{ .FilePath }}/key.pem
            - name: LEAF_NODE_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          volumeMounts:
            - name: credentials
              mountPath: "{{ .FilePath }}"
              readOnly: true
          command:
            - clustertree-cluster-manager
            - --multi-cluster-service=true
            - --v=4
            - --leader-elect-resource-namespace=kube-system
            - --kubeconfig={{ .FilePath }}/kubeconfig
      volumes:
        - name: credentials
          secret:
            secretName: {{ .Name }}-clustertree-cluster-manager
`
)

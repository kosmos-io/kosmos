package agent

const clusterlinkAgentDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  selector:
    matchLabels:
      app: clusterlink-agent
  template:
    metadata:
      labels:
        app: clusterlink-agent
    spec:
      tolerations:
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: clusterlink.io/exclude
                operator: DoesNotExist
      dnsPolicy: ClusterFirstWithHostNet
      containers:
      - name: clusterlink-agent
        securityContext:
          privileged: true
        image: {{ .ImageRepository }}/clusterlink-agent:{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
        - clusterlink-agent
        - --kubeconfig=/etc/clusterlink/kubeconfig
        env:
        - name: CLUSTER_NAME
          value: "{{ .ClusterName }}"  
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        resources:
          limits:
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 200Mi
        volumeMounts:
        - mountPath: /etc/clusterlink
          name: proxy-config
          readOnly: true
      terminationGracePeriodSeconds: 30
      hostNetwork: true
      volumes:
      - name: proxy-config
        secret:
          secretName: {{ .ProxyConfigMapName }}

`

// DaemonSetReplace is a struct to help to concrete
// the clusterlink-agent daemonset bytes with the daemonset template
type DaemonSetReplace struct {
	Namespace          string
	Name               string
	ProxyConfigMapName string
	ImageRepository    string
	Version            string
	ClusterName        string
}

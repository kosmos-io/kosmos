package manifest

const (
	ClusterlinkFloaterDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: clusterlink-floater
  namespace: {{ .Namespace }}
  labels:
    app: clusterlink-floater
spec:
  replicas: 1
  selector:
    matchLabels:
      app: clusterlink-floater
  template:
    metadata:
      labels:
        app: clusterlink-floater
    spec:
      hostNetwork: {{ .EnableHostNetwork }}
      serviceAccountName: clusterlink-floater
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kosmos.io/exclude
                operator: DoesNotExist
      containers:
      - name: floater
        image: {{ .ImageRepository }}/clusterlink-floater:{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
          - clusterlink-floater
        securityContext:
          privileged: true
        env: 
          - name: "PORT"
            value: "{{ .Port }}"
          - name: "ENABLE_ANALYSIS"
            value: "{{ .EnableAnalysis }}"
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - key: CriticalAddonsOnly
        operator: Exists
      - effect: NoExecute
        operator: Exists
`
)

type DaemonSetReplace struct {
	Namespace       string
	Name            string
	ImageRepository string
	Version         string
	Port            string

	EnableHostNetwork bool `default:"false"`
	EnableAnalysis    bool `default:"false"`
}

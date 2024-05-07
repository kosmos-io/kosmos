package etcd

const (
	// EtcdStatefulSet is  etcd StatefulSet manifest
	EtcdStatefulSet = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    virtualCluster-app: etcd
    app.kubernetes.io/managed-by: virtual-cluster-controller
  namespace: "{{ .Namespace }}"
  name: {{ .StatefulSetName }}
spec:
  replicas: {{ .Replicas }}
  serviceName: {{ .StatefulSetName }}
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      virtualCluster-app: etcd
  template:
    metadata:
      labels:
        virtualCluster-app: etcd
    spec:
      automountServiceAccountToken: false
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
                  - etcd
              topologyKey: kubernetes.io/hostname
      containers:
      - name: etcd
        image:  {{ .ImageRepository }}/etcd:{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
        - /usr/local/bin/etcd
        - --name=$(VIRTUAL_ETCD_NAME)
        - --listen-client-urls= https://0.0.0.0:{{ .EtcdListenClientPort }}
        - --listen-peer-urls=http://0.0.0.0:{{ .EtcdListenPeerPort }}
        - --advertise-client-urls=https://{{ .EtcdClientService }}.{{ .Namespace }}.svc.cluster.local:{{ .EtcdListenClientPort }}
        - --initial-cluster={{ .InitialCluster }}
        - --initial-cluster-state=new
        - --client-cert-auth=true
        - --trusted-ca-file=/etc/virtualcluster/pki/etcd/etcd-ca.crt
        - --cert-file=/etc/virtualcluster/pki/etcd/etcd-server.crt
        - --key-file=/etc/virtualcluster/pki/etcd/etcd-server.key
        - --data-dir=/var/lib/etcd
        - --snapshot-count=10000
        - --log-level=debug
        - --cipher-suites={{ .EtcdCipherSuites }}
        #- --peer-cert-file=/etc/virtualcluster/pki/etcd/etcd-server.crt
        #- --peer-client-cert-auth=true
        #- --peer-key-file=/etc/virtualcluster/pki/etcd/etcd-server.key
        #- --peer-trusted-ca-file=/etc/virtualcluster/pki/etcd/etcd-ca.crt
        env:
        - name: VIRTUAL_ETCD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        livenessProbe:
          exec:
            command:
            - /bin/sh
            - -ec
            - etcdctl get /registry --prefix --keys-only --endpoints https://127.0.0.1:{{ .EtcdListenClientPort }} --cacert=/etc/virtualcluster/pki/etcd/etcd-ca.crt --cert=/etc/virtualcluster/pki/etcd/etcd-server.crt --key=/etc/virtualcluster/pki/etcd/etcd-server.key
          failureThreshold: 3
          initialDelaySeconds: 600
          periodSeconds: 60
          successThreshold: 1
          timeoutSeconds: 10
        ports:
        - containerPort: {{ .EtcdListenClientPort }}
          name: client
          protocol: TCP
        - containerPort: {{ .EtcdListenPeerPort }}
          name: server
          protocol: TCP
        volumeMounts:
        - mountPath: /var/lib/etcd
          name: {{ .EtcdDataVolumeName }}
        - mountPath: /etc/virtualcluster/pki/etcd
          name: etcd-cert
      volumes:
      - name: etcd-cert
        secret:
          secretName: {{ .CertsSecretName }}
  volumeClaimTemplates:
  - metadata:
      name: etcd-data
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
      storageClassName: openebs-hostpath
`
)

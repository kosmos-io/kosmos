package calicoippool

import (
	"fmt"
	"strings"

	"github.com/kosmos.io/clusterlink/pkg/utils"
)

const (
	CalicoIPPool = `
apiVersion: crd.projectcalico.org/v1
kind: IPPool
metadata:
  name: {{ .Name }}
spec:
  cidr: {{ .IPPool }}
  natOutgoing: false
  disabled: true
  disableBGPExport: true
  vxlanMode: Never
  ipipMode: Never
`
	SERVICEIPType = "service"
	PODIPType     = "pod"
)

type IPPoolReplace struct {
	Name   string
	IPPool string
}

func genCalicoIPPoolName(cluster string, ipType string, ip string) string {
	suffix := strings.ReplaceAll(ip, ".", "-")
	suffix = strings.ReplaceAll(suffix, ":", "-")
	suffix = strings.ReplaceAll(suffix, "/", "-")
	return fmt.Sprintf("%s-%s-%s-%s", utils.ExternalIPPoolNamePrefix, cluster, ipType, suffix)
}

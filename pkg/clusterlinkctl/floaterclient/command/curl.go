package command

import (
	"fmt"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/utils"
)

type Curl struct {
	TargetIP string
}

func (c *Curl) GetCommandStr() string {
	// execute once
	if utils.IsIPv6(c.TargetIP) {
		return fmt.Sprintf("curl -k https://[%s]:8889/", c.TargetIP)
	}
	return fmt.Sprintf("curl -k https://%s:8889/", c.TargetIP)
}

func (c *Curl) ParseResult(result string) *Result {
	klog.Infof("curl result parser: %s", result)
	isSucceed := CommandSuccessed
	if result != "OK" {
		isSucceed = CommandFailed
	}
	return &Result{
		Status:    isSucceed,
		ResultStr: result,
	}
}

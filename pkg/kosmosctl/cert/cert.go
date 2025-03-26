package cert

import (
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/i18n"
)

func NewCmdCert() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "renew",
		Short: i18n.T("renew cert for kubenest cluster. "),
	}

	cmd.AddCommand(NewCmdRenewCert())
	return cmd
}

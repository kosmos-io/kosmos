package image

import (
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/i18n"
)

// NewCmdImage pull/push a kosmos offline installation package.
func NewCmdImage() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: i18n.T("pull and push kosmos offline installation package. "),
	}

	cmd.AddCommand(NewCmdPull())
	cmd.AddCommand(NewCmdPush())
	return cmd
}

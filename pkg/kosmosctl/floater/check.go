package floater

import (
	"fmt"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/floater/command"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/floater/netmap"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/version"
)

var checkExample = templates.Examples(i18n.T(`
        # Check single cluster network connectivity, e.g:
        kosmosctl check --src-kubeconfig ~/kubeconfig/src-kubeconfig
        
        # Check across clusters network connectivity, e.g:
        kosmosctl check --src-kubeconfig ~/kubeconfig/src-kubeconfig --dst-kubeconfig ~/kubeconfig/dst-kubeconfig
        
        # Check cluster network connectivity, if you need to specify a special image repository, e.g: 
        kosmosctl check -r ghcr.io/kosmos-io
`))

type CommandCheckOptions struct {
	Namespace          string
	ImageRepository    string
	DstImageRepository string
	Version            string

	Protocol    string
	PodWaitTime int
	Port        string
	HostNetwork bool

	KubeConfig    string
	SrcKubeConfig string
	DstKubeConfig string

	SrcFloater *Floater
	DstFloater *Floater
}

type PrintCheckData struct {
	command.Result
	SrcNodeName string
	DstNodeName string
	TargetIP    string
}

func NewCmdCheck() *cobra.Command {
	o := &CommandCheckOptions{
		Version: version.GetReleaseVersion().PatchRelease(),
	}
	cmd := &cobra.Command{
		Use:                   "check",
		Short:                 i18n.T("Check network connectivity between Kosmos clusters"),
		Long:                  "",
		Example:               checkExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete())
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run())
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", utils.DefaultNamespace, "Kosmos namespace.")
	flags.StringVarP(&o.ImageRepository, "image-repository", "r", utils.DefaultImageRepository, "Image repository.")
	flags.StringVarP(&o.DstImageRepository, "dst-image-repository", "", "", "Destination cluster image repository.")
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Absolute path to the host kubeconfig file.")
	flags.StringVar(&o.SrcKubeConfig, "src-kubeconfig", "", "Absolute path to the source cluster kubeconfig file.")
	flags.StringVar(&o.DstKubeConfig, "dst-kubeconfig", "", "Absolute path to the destination cluster kubeconfig file.")
	flags.BoolVar(&o.HostNetwork, "host-network", false, "Configure HostNetwork.")
	flags.StringVar(&o.Port, "port", "8889", "Port used by floater.")
	flags.IntVarP(&o.PodWaitTime, "pod-wait-time", "w", 30, "Time for wait pod(floater) launch.")
	flags.StringVar(&o.Protocol, "protocol", string(TCP), "Protocol for the network problem.")

	return cmd
}

func (o *CommandCheckOptions) Complete() error {
	if len(o.DstImageRepository) == 0 {
		o.DstImageRepository = o.ImageRepository
	}

	srcFloater := NewCheckFloater(o, false)
	if err := srcFloater.completeFromKubeConfigPath(o.SrcKubeConfig); err != nil {
		return err
	}
	o.SrcFloater = srcFloater

	if o.DstKubeConfig != "" {
		dstFloater := NewCheckFloater(o, true)
		if err := dstFloater.completeFromKubeConfigPath(o.DstKubeConfig); err != nil {
			return err
		}
		o.DstFloater = dstFloater
	}

	return nil
}

func (o *CommandCheckOptions) Validate() error {
	if len(o.Namespace) == 0 {
		return fmt.Errorf("namespace must be specified")
	}

	return nil
}

func (o *CommandCheckOptions) Run() error {
	var resultData []*PrintCheckData

	if err := o.SrcFloater.CreateFloater(); err != nil {
		return err
	}

	if o.DstKubeConfig != "" {
		if o.DstFloater.EnableHostNetwork {
			srcNodeInfos, err := o.SrcFloater.GetNodesInfo()
			if err != nil {
				return fmt.Errorf("get src cluster nodeInfos failed: %s", err)
			}

			if err = o.DstFloater.CreateFloater(); err != nil {
				return err
			}
			var dstNodeInfos []*FloatInfo
			dstNodeInfos, err = o.DstFloater.GetNodesInfo()
			if err != nil {
				return fmt.Errorf("get dist cluster nodeInfos failed: %s", err)
			}

			resultData = o.RunNative(srcNodeInfos, dstNodeInfos)
		} else {
			srcPodInfos, err := o.SrcFloater.GetPodInfo()
			if err != nil {
				return fmt.Errorf("get src cluster podInfos failed: %s", err)
			}

			if err = o.DstFloater.CreateFloater(); err != nil {
				return err
			}
			var dstPodInfos []*FloatInfo
			dstPodInfos, err = o.DstFloater.GetPodInfo()
			if err != nil {
				return fmt.Errorf("get dist cluster podInfos failed: %s", err)
			}

			resultData = o.RunRange(srcPodInfos, dstPodInfos)
		}
	} else {
		if o.SrcFloater.EnableHostNetwork {
			srcNodeInfos, err := o.SrcFloater.GetNodesInfo()
			if err != nil {
				return fmt.Errorf("get src cluster nodeInfos failed: %s", err)
			}
			resultData = o.RunNative(srcNodeInfos, srcNodeInfos)
		} else {
			srcPodInfos, err := o.SrcFloater.GetPodInfo()
			if err != nil {
				return fmt.Errorf("get src cluster podInfos failed: %s", err)
			}
			resultData = o.RunRange(srcPodInfos, srcPodInfos)
		}
	}

	o.PrintResult(resultData)

	if err := o.SrcFloater.RemoveFloater(); err != nil {
		return err
	}

	if o.DstKubeConfig != "" {
		if err := o.DstFloater.RemoveFloater(); err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandCheckOptions) RunRange(iPodInfos []*FloatInfo, jPodInfos []*FloatInfo) []*PrintCheckData {
	var resultData []*PrintCheckData

	if len(iPodInfos) > 0 && len(jPodInfos) > 0 {
		for _, iPodInfo := range iPodInfos {
			for _, jPodInfo := range jPodInfos {
				for _, ip := range jPodInfo.PodIPs {
					var targetIP string
					var err error
					var cmdResult *command.Result
					if o.DstFloater != nil {
						targetIP, err = netmap.NetMap(ip, o.DstFloater.CIDRsMap)
					} else {
						targetIP = ip
					}
					if err != nil {
						cmdResult = command.ParseError(err)
					} else {
						// ToDo RunRange && RunNative func support multiple commands, and the code needs to be optimized
						cmdObj := &command.Ping{
							TargetIP: targetIP,
						}
						cmdResult = o.SrcFloater.CommandExec(iPodInfo, cmdObj)
					}
					resultData = append(resultData, &PrintCheckData{
						*cmdResult,
						iPodInfo.NodeName, jPodInfo.NodeName, targetIP,
					})
				}
			}
		}
	}

	return resultData
}

func (o *CommandCheckOptions) RunNative(iNodeInfos []*FloatInfo, jNodeInfos []*FloatInfo) []*PrintCheckData {
	var resultData []*PrintCheckData

	if len(iNodeInfos) > 0 && len(jNodeInfos) > 0 {
		for _, iNodeInfo := range iNodeInfos {
			for _, jNodeInfo := range jNodeInfos {
				for _, ip := range jNodeInfo.NodeIPs {
					// ToDo RunRange && RunNative func support multiple commands, and the code needs to be optimized
					cmdObj := &command.Ping{
						TargetIP: ip,
					}
					cmdResult := o.SrcFloater.CommandExec(iNodeInfo, cmdObj)
					resultData = append(resultData, &PrintCheckData{
						*cmdResult,
						iNodeInfo.NodeName, jNodeInfo.NodeName, ip,
					})
				}
			}
		}
	}

	return resultData
}

func (o *CommandCheckOptions) PrintResult(resultData []*PrintCheckData) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"S/N", "SRC_NODE_NAME", "DST_NODE_NAME", "TARGET_IP", "RESULT"})

	tableException := tablewriter.NewWriter(os.Stdout)
	tableException.SetHeader([]string{"S/N", "SRC_NODE_NAME", "DST_NODE_NAME", "TARGET_IP", "RESULT"})

	for index, r := range resultData {
		// klog.Infof(fmt.Sprintf("%s %s %v", r.SrcNodeName, r.DstNodeName, r.IsSucceed))
		row := []string{strconv.Itoa(index + 1), r.SrcNodeName, r.DstNodeName, r.TargetIP, command.PrintStatus(r.Status)}
		if r.Status == command.CommandFailed {
			table.Rich(row, []tablewriter.Colors{
				{},
				{tablewriter.Bold, tablewriter.FgHiRedColor},
				{tablewriter.Bold, tablewriter.FgHiRedColor},
				{tablewriter.Bold, tablewriter.FgHiRedColor},
				{tablewriter.Bold, tablewriter.FgHiRedColor},
			})
		} else if r.Status == command.ExecError {
			tableException.Rich(row, []tablewriter.Colors{
				{},
				{tablewriter.Bold, tablewriter.FgCyanColor},
				{tablewriter.Bold, tablewriter.FgCyanColor},
				{tablewriter.Bold, tablewriter.FgCyanColor},
				{tablewriter.Bold, tablewriter.FgCyanColor},
			})
		} else {
			table.Rich(row, []tablewriter.Colors{
				{},
				{tablewriter.Bold, tablewriter.FgGreenColor},
				{tablewriter.Bold, tablewriter.FgGreenColor},
				{tablewriter.Bold, tablewriter.FgGreenColor},
				{tablewriter.Bold, tablewriter.FgGreenColor},
			})
		}
	}
	fmt.Println("")
	table.Render()
	fmt.Println("")
	tableException.Render()
}

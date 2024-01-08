package floater

import (
	"fmt"
	"os"
	"strconv"
	"sync"

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

	routinesMaxNum  int
	routineInfoChan chan routineInfo
	waitGroup       sync.WaitGroup
	resultDataChan  chan *PrintCheckData
}

type PrintCheckData struct {
	command.Result
	SrcNodeName string
	DstNodeName string
	TargetIP    string
}

type routineInfo struct {
	IInfo     *FloatInfo
	JInfo     *FloatInfo
	routineIp string
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
	flags.IntVarP(&o.routinesMaxNum, "routines-max-number", "", 5, "Number of goroutines to use.")

	o.routineInfoChan = make(chan routineInfo, o.routinesMaxNum)
	o.waitGroup = sync.WaitGroup{}
	o.resultDataChan = make(chan *PrintCheckData, o.routinesMaxNum)

	return cmd
}

func (o *CommandCheckOptions) Complete() error {
	if len(o.DstImageRepository) == 0 {
		o.DstImageRepository = o.ImageRepository
	}

	srcFloater := NewCheckFloater(o)
	if err := srcFloater.completeFromKubeConfigPath(o.SrcKubeConfig); err != nil {
		return err
	}
	o.SrcFloater = srcFloater

	dstFloater := NewCheckFloater(o)
	if err := dstFloater.completeFromKubeConfigPath(o.DstKubeConfig); err != nil {
		return err
	}
	o.DstFloater = dstFloater

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
	var resultDatas []*PrintCheckData

	if len(iPodInfos) > 0 && len(jPodInfos) > 0 {
		go o.toRoutineInfoChan(iPodInfos, jPodInfos)
	}

	for i := 0; i < o.routinesMaxNum; i++ {
		o.waitGroup.Add(1)
		go o.checkRange()
	}

	go func() {
		o.waitGroup.Wait()
		close(o.resultDataChan)
	}()

	for resultData := range o.resultDataChan {
		resultDatas = append(resultDatas, resultData)
	}
	return resultDatas
}

func (o *CommandCheckOptions) checkRange() {
	defer o.waitGroup.Done()
	for routineInfo := range o.routineInfoChan {
		var targetIP string
		var err error
		var cmdResult *command.Result
		if o.DstFloater != nil {
			targetIP, err = netmap.NetMap(routineInfo.routineIp, o.DstFloater.CIDRsMap)
		} else {
			targetIP = routineInfo.routineIp
		}
		if err != nil {
			cmdResult = command.ParseError(err)
		} else {
			// ToDo RunRange && RunNative func support multiple commands, and the code needs to be optimized
			cmdObj := &command.Ping{
				TargetIP: targetIP,
			}
			cmdResult = o.SrcFloater.CommandExec(routineInfo.IInfo, cmdObj)
		}
		resultData := &PrintCheckData{
			*cmdResult,
			routineInfo.IInfo.NodeName, routineInfo.JInfo.NodeName, targetIP,
		}
		o.resultDataChan <- resultData
	}
}

func (o *CommandCheckOptions) toRoutineInfoChan(iInfos []*FloatInfo, jInfos []*FloatInfo) {
	for _, iInfo := range iInfos {
		for _, jInfo := range jInfos {
			for _, ip := range jInfo.NodeIPs {
				routineIInfo := iInfo
				routineJInfo := jInfo
				routineIp := ip
				info := routineInfo{
					IInfo:     routineIInfo,
					JInfo:     routineJInfo,
					routineIp: routineIp,
				}
				o.routineInfoChan <- info
			}
		}
	}
	close(o.routineInfoChan)
}

func (o *CommandCheckOptions) RunNative(iNodeInfos []*FloatInfo, jNodeInfos []*FloatInfo) []*PrintCheckData {
	var resultDatas []*PrintCheckData

	if len(iNodeInfos) > 0 && len(jNodeInfos) > 0 {
		go o.toRoutineInfoChan(iNodeInfos, jNodeInfos)
	}

	for i := 0; i < o.routinesMaxNum; i++ {
		o.waitGroup.Add(1)
		go o.checkNative()
	}

	go func() {
		o.waitGroup.Wait()
		close(o.resultDataChan)
	}()

	for resultData := range o.resultDataChan {
		resultDatas = append(resultDatas, resultData)
	}
	return resultDatas
}

func (o *CommandCheckOptions) checkNative() {
	defer o.waitGroup.Done()
	for routineInfo := range o.routineInfoChan {
		// ToDo RunRange && RunNative func support multiple commands, and the code needs to be optimized
		cmdObj := &command.Ping{
			TargetIP: routineInfo.routineIp,
		}
		cmdResult := o.SrcFloater.CommandExec(routineInfo.IInfo, cmdObj)
		resultData := &PrintCheckData{
			*cmdResult,
			routineInfo.IInfo.NodeName, routineInfo.JInfo.NodeName, routineInfo.routineIp,
		}
		o.resultDataChan <- resultData
	}
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

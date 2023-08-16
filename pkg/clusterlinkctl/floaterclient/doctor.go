package floaterclient

import (
	"fmt"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/floaterclient/command"
	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/floaterclient/netmap"
	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/util"
	"github.com/kosmos.io/clusterlink/pkg/version"
)

type DoctorOptions struct {
	Namespace          string
	ImageRepository    string
	ImageRepositoryDst string
	Version            string

	Protocol    string
	PodWaitTime int
	Port        string
	HostNetwork bool

	SrcCluster     string
	DstCluster     string
	SrcKubeConfig  string
	DstKubeConfig  string
	HostKubeConfig string

	//SrcClusterNative    string
	//DstClusterNative    string
	//SrcKubeConfigNative string
	//DstKubeConfigNative string

	SrcFloater *Floater
	DstFloater *Floater

	ExtensionKubeClientSet apiextensionsclientset.Interface
}

type PrintData struct {
	command.Result
	SrcNodeName string
	DstNodeName string
	TargetIP    string
}

func CmdDoctor(parentCommand string) *cobra.Command {
	opts := &DoctorOptions{
		Version: version.GetReleaseVersion().PatchRelease(),
	}
	cmd := &cobra.Command{
		Use:                   "dr",
		Short:                 "Dr.link is an one-shot kubernetes network diagnose tool.",
		Long:                  "",
		Example:               "",
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.Run(); err != nil {
				return err
			}
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
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterRegistration,
		},
	}
	flags := cmd.Flags()

	flags.StringVarP(&opts.Namespace, "namespace", "n", "clusterlink-system", "Kubernetes namespace.")
	flags.StringVarP(&opts.ImageRepository, "image-repository", "r", "ghcr.io/kosmos-io/clusterlink", "Image repository.")
	flags.StringVarP(&opts.ImageRepositoryDst, "image-repository-dst", "", "", "Image repository.")

	flags.StringVar(&opts.HostKubeConfig, "host-kubeconfig", "", "Absolute path to the host kubeconfig file.")
	flags.StringVar(&opts.SrcCluster, "src-cluster", "", "Source cluster for the network diagnose.")
	flags.StringVar(&opts.DstCluster, "dst-cluster", "", "Destination cluster for the network diagnose.")
	flags.StringVar(&opts.SrcKubeConfig, "src-kubeconfig", "", "Absolute path to the source cluster kubeconfig file.")
	flags.StringVar(&opts.DstKubeConfig, "dst-kubeconfig", "", "Absolute path to the destination cluster kubeconfig file.")

	flags.BoolVar(&opts.HostNetwork, "host-network", false, "")
	flags.StringVar(&opts.Port, "port", "8889", "Port used by floater.")
	flags.IntVarP(&opts.PodWaitTime, "pod-wait-time", "w", 30, "Time for wait pod(floater) launch.")
	flags.StringVar(&opts.Protocol, "protocol", string(util.TCP), "Protocol for the network problem.")

	return cmd
}

// Complete completes all the required options
func (i *DoctorOptions) Complete() error {
	if len(i.ImageRepositoryDst) == 0 {
		i.ImageRepositoryDst = i.ImageRepository
	}

	if i.SrcCluster != "" {
		srcFloater := &Floater{
			Namespace:       i.Namespace,
			ImageRepository: i.ImageRepository,
			Version:         i.Version,
			PodWaitTime:     i.PodWaitTime,
			Port:            i.Port,
		}

		hc := &HostClusterHelper{
			Kubeconfig: i.HostKubeConfig,
		}
		if err := hc.Complete(); err != nil {
			return err
		}

		ResetConfig, KubeClientSet, CIDRsMap, err := hc.GetClusterInfo(i.SrcCluster)
		if err != nil {
			return err
		}
		srcFloater.KubeClientSet = KubeClientSet
		srcFloater.CIDRsMap = CIDRsMap
		srcFloater.KueResetConfig = ResetConfig

		if i.HostNetwork {
			srcFloater.EnableHostNetwork = true
		}

		if err = srcFloater.InitKubeClient(); err != nil {
			return err
		}
		i.SrcFloater = srcFloater
	} else {
		srcFloater := &Floater{
			Namespace:       i.Namespace,
			ImageRepository: i.ImageRepository,
			Version:         i.Version,
			PodWaitTime:     i.PodWaitTime,
			Port:            i.Port,
		}
		srcFloater.KubeConfig = i.SrcKubeConfig

		if i.HostNetwork {
			srcFloater.EnableHostNetwork = true
		}

		if err := srcFloater.InitKubeClient(); err != nil {
			return err
		}
		i.SrcFloater = srcFloater
	}

	if i.DstCluster != "" {
		dstFloater := &Floater{
			Namespace:       i.Namespace,
			ImageRepository: i.ImageRepositoryDst,
			Version:         i.Version,
			PodWaitTime:     i.PodWaitTime,
			Port:            i.Port,
		}

		hc := &HostClusterHelper{
			Kubeconfig: i.HostKubeConfig,
		}
		if err := hc.Complete(); err != nil {
			return err
		}

		ResetConfig, KubeClientSet, CIDRsMap, err := hc.GetClusterInfo(i.DstCluster)
		if err != nil {
			return err
		}
		dstFloater.KubeClientSet = KubeClientSet
		dstFloater.CIDRsMap = CIDRsMap
		dstFloater.KueResetConfig = ResetConfig

		if i.HostNetwork {
			dstFloater.EnableHostNetwork = true
		}

		if err = dstFloater.InitKubeClient(); err != nil {
			return err
		}
		i.DstFloater = dstFloater
	} else {
		dstFloater := &Floater{
			Namespace:       i.Namespace,
			ImageRepository: i.ImageRepositoryDst,
			Version:         i.Version,
			PodWaitTime:     i.PodWaitTime,
			Port:            i.Port,
		}
		dstFloater.KubeConfig = i.DstKubeConfig

		if i.HostNetwork {
			dstFloater.EnableHostNetwork = true
		}

		if err := dstFloater.InitKubeClient(); err != nil {
			return err
		}
		i.DstFloater = dstFloater
	}

	return nil
}

func (i *DoctorOptions) Run() error {
	var resultData []*PrintData

	if err := i.SrcFloater.RunInit(); err != nil {
		return err
	}

	if i.DstCluster != "" || i.DstKubeConfig != "" {
		if i.DstFloater.EnableHostNetwork {
			srcNodeInfos, err := i.SrcFloater.GetNodesInfo()
			if err != nil {
				return fmt.Errorf("get src cluster nodeInfos failed: %s", err)
			}

			if err = i.DstFloater.RunInit(); err != nil {
				return err
			}
			var dstNodeInfos []*FloaterInfo
			dstNodeInfos, err = i.DstFloater.GetNodesInfo()
			if err != nil {
				return fmt.Errorf("get dist cluster nodeInfos failed: %s", err)
			}

			resultData = i.RunNative(srcNodeInfos, dstNodeInfos)
		} else {
			srcPodInfos, err := i.SrcFloater.GetPodInfo()
			if err != nil {
				return fmt.Errorf("get src cluster podInfos failed: %s", err)
			}

			if err = i.DstFloater.RunInit(); err != nil {
				return err
			}
			var dstPodInfos []*FloaterInfo
			dstPodInfos, err = i.DstFloater.GetPodInfo()
			if err != nil {
				return fmt.Errorf("get dist cluster podInfos failed: %s", err)
			}

			resultData = i.RunRange(srcPodInfos, dstPodInfos)
		}
	} else {
		if i.SrcFloater.EnableHostNetwork {
			srcNodeInfos, err := i.SrcFloater.GetNodesInfo()
			if err != nil {
				return fmt.Errorf("get src cluster nodeInfos failed: %s", err)
			}
			resultData = i.RunNative(srcNodeInfos, srcNodeInfos)
		} else {
			srcPodInfos, err := i.SrcFloater.GetPodInfo()
			if err != nil {
				return fmt.Errorf("get src cluster podInfos failed: %s", err)
			}
			resultData = i.RunRange(srcPodInfos, srcPodInfos)
		}
	}

	i.PrintResult(resultData)

	return nil
}

func (i *DoctorOptions) RunRange(iPodInfos []*FloaterInfo, jPodInfos []*FloaterInfo) []*PrintData {
	var resultData []*PrintData

	if len(iPodInfos) > 0 && len(jPodInfos) > 0 {
		for _, iPodInfo := range iPodInfos {
			for _, jPodInfo := range jPodInfos {
				for _, ip := range jPodInfo.PodIPs {
					var targetIP string
					var err error
					var cmdResult *command.Result
					if i.DstFloater != nil {
						targetIP, err = netmap.NetMap(ip, i.DstFloater.CIDRsMap)
					} else {
						targetIP = ip
					}
					if err != nil {
						cmdResult = command.ParseError(err)
					} else {
						// ToDo RunRange和RunNative函数支持多命令，代码待优化@wangqi
						cmdObj := &command.Ping{
							TargetIP: targetIP,
						}
						cmdResult = i.SrcFloater.CommandExec(iPodInfo, cmdObj)
					}
					resultData = append(resultData, &PrintData{
						*cmdResult,
						iPodInfo.NodeName, jPodInfo.NodeName, targetIP,
					})
				}
			}
		}
	}

	return resultData
}

func (i *DoctorOptions) RunNative(iNodeInfos []*FloaterInfo, jNodeInfos []*FloaterInfo) []*PrintData {
	var resultData []*PrintData

	if len(iNodeInfos) > 0 && len(jNodeInfos) > 0 {
		for _, iNodeInfo := range iNodeInfos {
			for _, jNodeInfo := range jNodeInfos {
				for _, ip := range jNodeInfo.NodeIPs {
					// ToDo RunRange和RunNative函数支持多命令，代码待优化@wangqi
					cmdObj := &command.Ping{
						TargetIP: ip,
					}
					cmdResult := i.SrcFloater.CommandExec(iNodeInfo, cmdObj)
					resultData = append(resultData, &PrintData{
						*cmdResult,
						iNodeInfo.NodeName, jNodeInfo.NodeName, ip,
					})
				}
			}
		}
	}

	return resultData
}

func (i *DoctorOptions) PrintResult(resultData []*PrintData) {
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

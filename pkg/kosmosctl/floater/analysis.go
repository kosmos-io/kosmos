package floater

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/version"
)

var analysisExample = templates.Examples(i18n.T(`
        # Analysis cluster network, e.g: 
        kosmosctl analysis cluster --name cluster-name --kubeconfig ~/kubeconfig/cluster-kubeconfig
`))

type CommandAnalysisOptions struct {
	Namespace       string
	Name            string
	ImageRepository string
	Version         string
	KubeConfig      string
	Context         string

	Port        string
	PodWaitTime int
	GenGraph    bool
	GenPath     string

	Floater       *Floater
	DynamicClient *dynamic.DynamicClient

	AnalysisResult []*PrintAnalysisData
}

type PrintAnalysisData struct {
	ClusterName     string
	ClusterNodeName string
	ParameterType   string
	AnalyzeResult   string
}

func NewCmdAnalysis(f ctlutil.Factory) *cobra.Command {
	o := &CommandAnalysisOptions{}

	cmd := &cobra.Command{
		Use:                   "analysis",
		Short:                 i18n.T("Analysis network connectivity between Kosmos clusters"),
		Long:                  "",
		Example:               analysisExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f))
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run(args))
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", utils.DefaultNamespace, "Kosmos namespace.")
	flags.StringVarP(&o.ImageRepository, "image-repository", "r", utils.DefaultImageRepository, "Image repository.")
	flags.StringVar(&o.Name, "name", "", "Specify the name of the resource to analysis.")
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Absolute path to the cluster kubeconfig file.")
	flags.StringVar(&o.Context, "context", "", "The name of the kubeconfig context.")
	flags.StringVar(&o.Port, "port", utils.DefaultPort, "Port used by floater.")
	flags.IntVarP(&o.PodWaitTime, "pod-wait-time", "w", utils.DefaultWaitTime, "Time for wait pod(floater) launch.")
	flags.BoolVar(&o.GenGraph, "gen-graph", false, "Configure generate network analysis graph.")
	flags.StringVar(&o.GenPath, "gen-path", "~/", "Configure save path for generate network analysis graph.")

	return cmd
}

func (o *CommandAnalysisOptions) Complete(f ctlutil.Factory) error {
	c, err := f.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("kosmosctl analysis complete error, generate rest config failed: %v", err)
	}
	o.DynamicClient, err = dynamic.NewForConfig(c)
	if err != nil {
		return fmt.Errorf("kosmosctl analysis complete error, generate dynamic client failed: %s", err)
	}

	if len(o.Version) == 0 {
		o.Version = version.GetReleaseVersion().PatchRelease()
	}

	af := NewAnalysisFloater(o)
	if err = af.completeFromKubeConfigPath(o.KubeConfig, o.Context); err != nil {
		return err
	}
	o.Floater = af

	return nil
}

func (o *CommandAnalysisOptions) Validate() error {
	if len(o.Namespace) == 0 {
		return fmt.Errorf("kosmosctl analysis validate error, namespace is not valid")
	}

	if len(o.Name) == 0 {
		return fmt.Errorf("kosmosctl analysis validate error, name is not valid")
	}

	return nil
}

func (o *CommandAnalysisOptions) Run(args []string) error {
	switch args[0] {
	case "cluster":
		err := o.runCluster()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandAnalysisOptions) runCluster() error {
	if err := o.Floater.CreateFloater(); err != nil {
		return err
	}

	sysNodeConfigs, err := o.Floater.GetSysNodeConfig()
	if err != nil {
		return fmt.Errorf("get cluster nodeConfigInfos failed: %s", err)
	}

	for _, sysNodeConfig := range sysNodeConfigs {
		var obj unstructured.Unstructured
		var nodeConfig v1alpha1.NodeConfig
		nodeConfigs, err := o.DynamicClient.Resource(util.NodeConfigGVR).List(context.TODO(), metav1.ListOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("get nodeconfig failed: %v", err)
		}
		for _, n := range nodeConfigs.Items {
			if sysNodeConfig.NodeName == n.GetName() {
				obj = n
				break
			}
		}
		jsonData, err := obj.MarshalJSON()
		if err != nil {
			return fmt.Errorf("marshal nodeconfig failed: %v", err)
		}
		err = json.Unmarshal(jsonData, &nodeConfig)
		if err != nil {
			return fmt.Errorf("unmarshal nodeconfig failed: %v", err)
		}
		o.analysisNodeConfig(sysNodeConfig.NodeName, sysNodeConfig.NodeConfigSpec, nodeConfig.Spec)
	}

	o.PrintResult(o.AnalysisResult)

	return o.Floater.RemoveFloater()
}

func (o *CommandAnalysisOptions) analysisNodeConfig(nodeName string, nc1 v1alpha1.NodeConfigSpec, nc2 v1alpha1.NodeConfigSpec) {
	analyzeType1 := reflect.TypeOf(nc1)
	analyzeType2 := reflect.TypeOf(nc2)

	for i := 0; i < analyzeType1.NumField(); i++ {
		r := &PrintAnalysisData{
			ClusterName:     o.Name,
			ClusterNodeName: nodeName,
		}
		field1 := analyzeType1.Field(i)
		field2 := analyzeType2.Field(i)

		if field1.Type != field2.Type {
			r.ParameterType = field1.Name
			r.AnalyzeResult = "false"
			o.AnalysisResult = append(o.AnalysisResult, r)
			continue
		}

		value1 := reflect.ValueOf(nc1).Field(i)
		value2 := reflect.ValueOf(nc2).Field(i)

		if !reflect.DeepEqual(value1.Interface(), value2.Interface()) {
			r.ParameterType = field1.Name
			r.AnalyzeResult = "false"
			o.AnalysisResult = append(o.AnalysisResult, r)
			continue
		}

		r.ParameterType = field1.Name
		r.AnalyzeResult = "true"
		o.AnalysisResult = append(o.AnalysisResult, r)
	}
}

func (o *CommandAnalysisOptions) PrintResult(resultData []*PrintAnalysisData) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"S/N", "CLUSTER_NAME", "CLUSTER_NODE_NAME", "PARAMETER_TYPE", "ANALYZE_RESULT"})

	tableException := tablewriter.NewWriter(os.Stdout)
	tableException.SetHeader([]string{"S/N", "CLUSTER_NAME", "CLUSTER_NODE_NAME", "PARAMETER_TYPE", "ANALYZE_RESULT"})

	for index, r := range resultData {
		row := []string{strconv.Itoa(index + 1), r.ClusterName, r.ClusterNodeName, r.ParameterType, r.AnalyzeResult}
		if r.AnalyzeResult == "false" {
			tableException.Rich(row, []tablewriter.Colors{
				{},
				{tablewriter.Bold, tablewriter.FgHiRedColor},
				{tablewriter.Bold, tablewriter.FgHiRedColor},
				{tablewriter.Bold, tablewriter.FgHiRedColor},
				{tablewriter.Bold, tablewriter.FgHiRedColor},
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

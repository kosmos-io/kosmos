package image

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/namespaces"
	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"k8s.io/klog"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

var PullExample = templates.Examples(i18n.T(`
     # Pull and save images with default config, e.g:
     kosmosctl image pull

     # Pull and save images with custom config, e.g:
     kosmosctl image pull --kosmos-version=[kosmos-image-version] --coredns-version=[coredns-image-version] --eps-version=[eps-image-version] --output=[output-dir] --containerd-runtime=[container-runtime]
`))

type CommandPullOptions struct {
	Output              string
	ImageList           string
	ContainerRuntime    string
	KosmosImageVersion  string
	CorednsImageVersion string
	EpsImageVersion     string
	ContainerdNamespace string
	Context             context.Context
	DockerClient        *docker.Client
	ContainerdClient    *containerd.Client
}

func NewCmdPull() *cobra.Command {
	o := &CommandPullOptions{}
	cmd := &cobra.Command{
		Use:                   "pull",
		Short:                 i18n.T("pull a kosmos offline installation package. "),
		Long:                  "",
		Example:               PullExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete())
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run())
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&o.ImageList, "image-list", "l", "", "Image list of kosmos. ")
	flags.StringVarP(&o.Output, "output", "o", "", "Path to a output path, default path is current dir")
	flags.StringVarP(&o.ContainerRuntime, "containerd-runtime", "c", utils.DefaultContainerRuntime, "Type of container runtime(docker or containerd), docker is used by default .")
	flags.StringVarP(&o.ContainerdNamespace, "containerd-namespace", "n", utils.DefaultContainerdNamespace, "Namespace of containerd. ")
	flags.StringVarP(&o.KosmosImageVersion, "kosmos-version", "", utils.DefaultVersion, "Image list of kosmos. ")
	flags.StringVarP(&o.CorednsImageVersion, "coredns-version", "", utils.DefaultVersion, "Image list of kosmos. ")
	flags.StringVarP(&o.EpsImageVersion, "eps-version", "", utils.DefaultVersion, "Image list of kosmos. ")
	return cmd
}

func (o *CommandPullOptions) Complete() (err error) {
	if len(o.Output) == 0 {
		currentPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("current path can not find: %s", err)
		}
		o.Output = currentPath
	}

	switch o.ContainerRuntime {
	case utils.Containerd:
		o.ContainerdClient, err = containerd.New(utils.DefaultContainerdSockAddress)
		if err != nil {
			return fmt.Errorf("init containerd client failed: %s", err)
		}
	default:
		o.DockerClient, err = docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
		if err != nil {
			return fmt.Errorf("init docker client failed: %s", err)
		}
	}

	o.Context = namespaces.WithNamespace(context.TODO(), o.ContainerdNamespace)
	return nil
}

func (o *CommandPullOptions) Validate() error {
	return nil
}

func (o *CommandPullOptions) Run() error {
	// 1. pull image from public registry
	klog.V(4).Info("Start pulling images ...")
	imageList, err := o.PullImage()
	if err != nil {
		klog.V(4).Infof("image pull failed: %s", err)
		return err
	}
	klog.V(4).Info("kosmos images have been pulled successfully. ")

	// 2. save image to *.tar.gz
	klog.V(4).Info("Start saving images ...")
	err = o.SaveImage(imageList)
	if err != nil {
		klog.V(4).Infof("image save failed: %s", err)
		return err
	}
	klog.V(4).Info("kosmos-io.tar.gz has been saved successfully. ")
	return nil
}

func (o *CommandPullOptions) PullImage() (imageList []string, err error) {
	if len(o.ImageList) != 0 {
		// pull images from image-list.txt
		imageList, err = o.PullFromImageList()
		if err != nil {
			return nil, err
		}
	} else {
		// pull images with specific version
		imageList, err = o.PullWithSpecificVersion()
		if err != nil {
			return nil, err
		}
	}
	return imageList, nil
}

func (o *CommandPullOptions) SaveImage(imageList []string) (err error) {
	switch o.ContainerRuntime {
	case utils.Containerd:
		err = o.ContainerdExport(imageList)
		if err != nil {
			return err
		}
	default:
		err = o.DockerSave(imageList)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *CommandPullOptions) PullFromImageList() (imageList []string, err error) {
	var imageName string
	file, err := os.Open(o.ImageList)
	if err != nil {
		return nil, fmt.Errorf("read image list failed: %v", err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			klog.Errorf("fail close failed: %s", err)
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		imageName = scanner.Text()
		err = o.PullCommand(imageName)
		if err != nil {
			return nil, err
		}
		imageList = append(imageList, imageName)
	}
	return imageList, nil
}

func (o *CommandPullOptions) PullWithSpecificVersion() (imageList []string, err error) {
	var imageName string
	for _, name := range utils.ImageList {
		switch name {
		case utils.Coredns:
			imageName = fmt.Sprintf("%s:%s", name, o.CorednsImageVersion)
		case utils.EpsProbePlugin:
			imageName = fmt.Sprintf("%s:%s", name, o.EpsImageVersion)
		default:
			imageName = fmt.Sprintf("%s:%s", name, o.KosmosImageVersion)
		}
		err := o.PullCommand(imageName)
		if err != nil {
			return nil, err
		}
		imageList = append(imageList, imageName)
	}
	return imageList, nil
}

func (o *CommandPullOptions) PullCommand(imageName string) (err error) {
	switch o.ContainerRuntime {
	case utils.Containerd:
		err = o.ContainerdPull(imageName)
		if err != nil {
			return err
		}
	default:
		err = o.DockerPull(imageName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *CommandPullOptions) DockerPull(imageName string) (err error) {
	reader, err := o.DockerClient.ImagePull(context.Background(), imageName, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("docker pull %s failed: %s", imageName, err)
	}
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return err
	}
	klog.V(4).Infof("docker pull %s successfully.", imageName)
	return nil
}

func (o *CommandPullOptions) DockerSave(imageList []string) (err error) {
	outputPath := fmt.Sprintf("%s/%s", o.Output, utils.DefaultTarName)
	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return fmt.Errorf("open file %s failed: %s", outputPath, err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			klog.Errorf("file close failed: %s", err)
		}
	}()

	saveResponse, err := o.DockerClient.ImageSave(context.Background(), imageList)
	if err != nil {
		return fmt.Errorf("docker save images failed: %s", err)
	}

	if _, err = io.Copy(file, saveResponse); err != nil {
		return fmt.Errorf("io.Copy failed: %s", err)
	}
	return nil
}

func (o *CommandPullOptions) ContainerdPull(imageName string) (err error) {
	opts := []containerd.RemoteOpt{
		containerd.WithPullUnpack,
	}
	image, err := o.ContainerdClient.Pull(o.Context, imageName, opts...)
	if err != nil {
		return fmt.Errorf("ctr image pull %s failed: %s", imageName, err)
	}
	klog.V(4).Infof("ctr image pull %s successfully.", image.Name())
	return nil
}

func (o *CommandPullOptions) ContainerdExport(imageList []string) (err error) {
	outputPath := fmt.Sprintf("%s/%s", o.Output, utils.DefaultTarName)
	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("open file %s failed: %s", outputPath, err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			klog.Errorf("file close failed: %s", err)
		}
	}()

	imageStore := o.ContainerdClient.ImageService()
	var exportOpts []archive.ExportOpt
	for _, imageName := range imageList {
		if len(imageName) == 0 {
			continue
		}
		klog.V(4).Infof("imageName: %s", imageName)
		exportOpts = append(exportOpts, archive.WithImage(imageStore, imageName))
	}

	err = o.ContainerdClient.Export(o.Context, file, exportOpts...)
	if err != nil && outputPath != "" {
		if err1 := os.Remove(outputPath); err1 != nil {
			return fmt.Errorf("os,Remove failed: %s", err1)
		}
		return fmt.Errorf("ctr image export failed: %s", err)
	}
	return nil
}

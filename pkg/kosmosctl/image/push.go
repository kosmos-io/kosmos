package image

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/remotes"
	docker2 "github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	docker "github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

var PushExample = templates.Examples(i18n.T(`
		# Push images for ./*.tar.gz to private-registry, e.g:
		kosmoscrl image push --artifact=[*.tar.gz] --private-registry=[private-registry-name]

		# Push images for ./*.tar.gz to private-registry which need to logged in, e.g:
		kosmoscrl image push --artifact=[*.tar.gz] --username=[registry-username] --private-registry=[private-registry-name]
`))

type CommandPushOptions struct {
	UserName            string
	PassWord            string
	PrivateRegistry     string
	ContainerRuntime    string
	ContainerdNamespace string
	ImageList           string
	Artifact            string
	Context             context.Context
	DockerClient        *docker.Client
	ContainerdClient    *containerd.Client
}

func NewCmdPush() *cobra.Command {
	o := &CommandPushOptions{}
	cmd := &cobra.Command{
		Use:                   "push",
		Short:                 i18n.T("push images from *.tar.gz to private registry. "),
		Long:                  "",
		Example:               PushExample,
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
	flags.StringVarP(&o.ImageList, "image-list", "d", "", "Path of image-list.txt.  ")
	flags.StringVarP(&o.Artifact, "artifact", "a", "", "Path of kosmos-io.tar.gz ")
	flags.StringVarP(&o.UserName, "username", "u", "", "Username to private registry. ")
	flags.StringVarP(&o.PrivateRegistry, "private-registry", "r", "", "private registry. ")
	flags.StringVarP(&o.ContainerRuntime, "containerd-runtime", "c", utils.DefaultContainerRuntime, "Type of container runtime(docker or containerd), docker is used by default .")
	flags.StringVarP(&o.ContainerdNamespace, "containerd-namespace", "n", utils.DefaultContainerdNamespace, "Namespace of containerd. ")
	return cmd
}

func (o *CommandPushOptions) Complete() (err error) {
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

	if len(o.UserName) != 0 {
		klog.V(4).Info("please enter password of registry: ")
		o.PassWord, err = o.passwordPrompt()
		if err != nil {
			return fmt.Errorf("enter password failed: %s", err)
		}
	}

	o.Context = namespaces.WithNamespace(context.TODO(), o.ContainerdNamespace)

	return nil
}

func (o *CommandPushOptions) Validate() (err error) {
	if len(o.Artifact) == 0 {
		return fmt.Errorf("artifact path can not be empty. ")
	}

	if len(o.UserName) == 0 {
		return fmt.Errorf("userName of registry can not be empty. ")
	}

	if len(o.PrivateRegistry) == 0 {
		return fmt.Errorf("private registry can not be empty. ")
	}

	return nil
}

func (o *CommandPushOptions) Run() error {
	// 1. load image from *.tar.gz
	klog.V(4).Info("Start loading images ...")
	imageList, err := o.LoadImage()
	if err != nil {
		klog.Infof("image load failed: %s", err)
		return err
	}
	klog.V(4).Info("kosmos images have been loaded successfully. ")

	// 2. push image to private registry
	klog.V(4).Info("Start pushing images ...")
	err = o.PushImage(imageList)
	if err != nil {
		klog.V(4).Infof("image push failed: %s", err)
		return nil
	}
	klog.V(4).Info("kosmos images have been pushed successfully. ")
	return nil
}

func (o *CommandPushOptions) LoadImage() (imageList []string, err error) {
	switch o.ContainerRuntime {
	case utils.Containerd:
		imageList, err = o.ContainerdImport()
		if err != nil {
			return nil, err
		}
	default:
		imageList, err = o.DockerLoad()
		if err != nil {
			return nil, err
		}
	}
	return imageList, nil
}

func (o *CommandPushOptions) PushImage(imageList []string) error {
	if len(o.ImageList) != 0 {
		// push images from image-list.txt
		file, err := os.Open(o.ImageList)
		if err != nil {
			return fmt.Errorf("read image list, err: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				klog.Errorf("file close failed: %s", err)
			}
		}()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			imageName := scanner.Text()
			err = o.PushCommand(imageName)
			if err != nil {
				return err
			}
		}
	} else {
		// push images with specific version
		for _, imageName := range imageList {
			if len(imageName) == 0 {
				continue
			}
			imageName = strings.TrimSpace(imageName)
			err := o.PushCommand(imageName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *CommandPushOptions) PushCommand(imageName string) (err error) {
	splits := strings.Split(imageName, "/")
	imageTagName := fmt.Sprintf("%s/%s", o.PrivateRegistry, splits[len(splits)-1])
	switch o.ContainerRuntime {
	case utils.Containerd:
		err = o.ContainerdTag(imageName, imageTagName)
		if err != nil {
			return err
		}
		err = o.ContainerdPush(imageTagName)
		if err != nil {
			return err
		}
	default:
		err = o.DockerTag(imageName, imageTagName)
		if err != nil {
			return err
		}
		err = o.DockerPush(imageTagName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *CommandPushOptions) DockerLoad() (imageList []string, err error) {
	file, err := os.Open(o.Artifact)
	if err != nil {
		return nil, fmt.Errorf("open %s failed: %s", o.Artifact, err)
	}
	defer file.Close()
	imageLoadResponse, err := o.DockerClient.ImageLoad(context.Background(), file, true)
	if err != nil {
		return nil, fmt.Errorf("docker load failed: %s", err)
	}

	body, err := io.ReadAll(imageLoadResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("io.Read failed: %s", err)
	}

	strSlice := strings.Split(string(body), "\n")
	for _, str := range strSlice {
		if len(str) == 0 {
			continue
		}
		imageParts := strings.Split(str, ":")
		imageVersion := imageParts[len(imageParts)-1]

		var imageName string
		if strings.Contains(imageVersion, utils.DefaultVersion) {
			imageName = fmt.Sprintf("%s:%s", imageParts[len(imageParts)-2], utils.DefaultVersion)
		} else if strings.Contains(imageVersion, "v") {
			regex := regexp.MustCompile(`v\d+\.\d+\.\d+`)
			imageVersionMatch := regex.FindString(imageVersion)
			imageName = fmt.Sprintf("%s:%s", imageParts[len(imageParts)-2], imageVersionMatch)
		}

		if len(imageName) == 0 {
			continue
		}
		imageList = append(imageList, imageName)
	}
	return imageList, nil
}

func (o *CommandPushOptions) DockerTag(imageSourceName, imageTargetName string) (err error) {
	err = o.DockerClient.ImageTag(context.Background(), imageSourceName, imageTargetName)
	if err != nil {
		return fmt.Errorf("docker tag %s %s failed: %s", imageSourceName, imageTargetName, err)
	}
	return nil
}

func (o *CommandPushOptions) DockerPush(imageName string) (err error) {
	var result io.ReadCloser

	authConfig := registry.AuthConfig{
		Username: o.UserName,
		Password: o.PassWord,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return fmt.Errorf("json marshal failed: %s", err)
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	result, err = o.DockerClient.ImagePush(context.Background(), imageName, types.ImagePushOptions{RegistryAuth: authStr})
	if err != nil {
		return fmt.Errorf("docker push failed: %s", err)
	}

	body, err := io.ReadAll(result)
	if err != nil {
		klog.Info(err)
		return fmt.Errorf(" ioutil Readall failed: %s", err)
	}
	klog.V(4).Infof(string(body))
	klog.V(4).Infof("docker push %s successfully.", imageName)
	return nil
}

func (o *CommandPushOptions) ContainerdImport() (imageList []string, err error) {
	file, err := os.Open(o.Artifact)
	if err != nil {
		return nil, fmt.Errorf("open %s failed: %s", o.Artifact, err)
	}
	defer file.Close()

	images, err := o.ContainerdClient.Import(o.Context, file)
	if err != nil {
		return nil, fmt.Errorf("cre image import failed: %s", err)
	}

	for _, image := range images {
		imageList = append(imageList, image.Name)
		klog.Infof(" ctr image import %s successfully.", image.Name)
	}
	return imageList, nil
}

func (o *CommandPushOptions) ContainerdTag(imageSourceName, imageTargetName string) (err error) {
	target, err := refdocker.ParseDockerRef(imageTargetName)
	if err != nil {
		return fmt.Errorf("parse docekr ref failed: %s", err)
	}

	ctx, done, err := o.ContainerdClient.WithLease(o.Context)
	if err != nil {
		return fmt.Errorf("with lease failed: %s", err)
	}
	defer func() {
		if err = done(ctx); err != nil {
			klog.Errorf("done failed: %s", err)
		}
	}()

	imageService := o.ContainerdClient.ImageService()
	image, err := imageService.Get(ctx, imageSourceName)
	if err != nil {
		return fmt.Errorf("imageService get image failed: %s", err)
	}
	image.Name = target.String()
	if _, err = imageService.Create(ctx, image); err != nil {
		if errdefs.IsAlreadyExists(err) {
			if err = imageService.Delete(ctx, image.Name); err != nil {
				return fmt.Errorf("imageService delete image failed: %s", err)
			}
			if _, err = imageService.Create(ctx, image); err != nil {
				return fmt.Errorf("imageService create image failed: %s", err)
			}
		} else {
			return fmt.Errorf("ctr image tag %s %s failed: %s", imageSourceName, imageTargetName, err)
		}
	}
	return nil
}

func (o *CommandPushOptions) ContainerdPush(imageName string) (err error) {
	image, err := o.ContainerdClient.GetImage(o.Context, imageName)
	if err != nil {
		return fmt.Errorf("get image failed: %s", err)
	}
	resolver, err := o.GetResolver()
	if err != nil {
		return fmt.Errorf("get resolver failed: %s", err)
	}

	options := []containerd.RemoteOpt{
		containerd.WithResolver(resolver),
	}
	err = o.ContainerdClient.Push(o.Context, imageName, image.Target(), options...)
	if err != nil {
		return fmt.Errorf("ctr image push %s failed: %s", imageName, err)
	}
	klog.V(4).Infof("ctr image push %s successfully.", imageName)
	return nil
}

func (o *CommandPushOptions) GetResolver() (remotes.Resolver, error) {
	var PushTracker = docker2.NewInMemoryTracker()
	options := docker2.ResolverOptions{
		Tracker: PushTracker,
	}

	hostOptions := config.HostOptions{}
	hostOptions.Credentials = func(host string) (string, string, error) {
		return o.UserName, o.PassWord, nil
	}

	hostOptions.DefaultTLS = &tls.Config{MinVersion: tls.VersionTLS13}
	options.Hosts = config.ConfigureHosts(o.Context, hostOptions)

	return docker2.NewResolver(options), nil
}

func (o *CommandPushOptions) passwordPrompt() (string, error) {
	c := console.Current()
	defer func() {
		if err := c.Reset(); err != nil {
			klog.Errorf("c.Reset failed: %s", err)
		}
	}()

	if err := c.DisableEcho(); err != nil {
		return "", fmt.Errorf("failed to disable echo: %w", err)
	}

	line, _, err := bufio.NewReader(c).ReadLine()
	if err != nil {
		return "", fmt.Errorf("failed to read line: %w", err)
	}
	return string(line), nil
}

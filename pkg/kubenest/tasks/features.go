package tasks

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewFeaturesTask() workflow.Task {
	return workflow.Task{
		Name:        "features",
		Run:         runFeatures,
		RunSubTasks: true,
	}
}

func doExportEtcd(data InitData, components []ComponentConfig) error {
	component, err := findComponent(components, constants.ExposeEtcd)
	if err != nil {
		return err
	}

	dynamicClient := data.DynamicClient()
	imageRepository, _ := util.GetImageMessage()

	klog.V(2).Infof("Deploy component %s", component.Name)
	templatedMapping := map[string]interface{}{
		"Namespace":            data.GetNamespace(),
		"Name":                 data.GetName(),
		"ImageRepository":      imageRepository,
		"EtcdListenClientPort": constants.EtcdListenClientPort,
	}
	for k, v := range data.PluginOptions() {
		templatedMapping[k] = v
	}

	err = applyYMLTemplate(dynamicClient, component.Path, templatedMapping)
	if err != nil {
		return err
	}

	return nil
}

func findComponent(components []ComponentConfig, name string) (ComponentConfig, error) {
	for _, compcomponent := range components {
		if compcomponent.Name == name {
			return compcomponent, nil
		}
	}

	return ComponentConfig{}, fmt.Errorf("component %s not found", name)
}

func runFeatures(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("features task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[features] Running coreDns task", "virtual cluster", klog.KObj(data))

	vc := data.VirtualCluster()
	features := strings.Split(vc.Annotations[constants.EnabledFeaturesAnnotation], ",")

	components, err := getManifestComponentsConfig(data.RemoteClient(), constants.FeatureComponents)
	if err != nil {
		return err
	}

	for _, f := range features {
		switch strings.TrimSpace(f) {
		case constants.ExposeEtcd:
			if err := doExportEtcd(data, components); err != nil {
				return err
			}
		default:
			klog.V(2).InfoS("[features] Unknown feature", "feature", f)
		}
	}

	return nil
}

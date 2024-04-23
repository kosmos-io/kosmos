package vcrnodepoolcontroller

import (
	"gopkg.in/yaml.v3"
)

type NodePoolMapItem struct {
	Address string            `yaml:"address"`
	Labels  map[string]string `yaml:"labels"`
	Cluster string            `yaml:"cluster"`
	State   string            `yaml:"state"`
}

type NodeItem struct {
	NodePoolMapItem
	Name string `yaml:"-"`
}

func ConvertYamlToNodeItem(yamlStr string) (map[string]NodeItem, error) {
	nodepoolMap := map[string]NodeItem{}

	nodepoolItem, err := ConvertYamlToNodePoolItem(yamlStr)
	if err != nil {
		return nil, err
	}

	for k, v := range nodepoolItem {
		nodepoolMap[k] = NodeItem{
			NodePoolMapItem: v,
			Name:            k,
		}
	}

	return nodepoolMap, nil
}

func ConvertYamlToNodePoolItem(yamlStr string) (map[string]NodePoolMapItem, error) {
	nodepoolItem := map[string]NodePoolMapItem{}
	err := yaml.Unmarshal([]byte(yamlStr), &nodepoolItem)
	if err != nil {
		return nil, err
	}
	return nodepoolItem, nil
}

func ConvertNodePoolItemToYaml(nodepoolItem map[string]NodePoolMapItem) ([]byte, error) {
	yamlStr, err := yaml.Marshal(nodepoolItem)
	if err != nil {
		return nil, err
	}
	return yamlStr, nil
}

// controller task
// TODO： free node need join to  host cluster
// TODO:  check orphan node

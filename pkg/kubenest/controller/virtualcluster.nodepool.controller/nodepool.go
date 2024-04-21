package vcrnodepoolcontroller

import (
	"encoding/json"
)

type NodePoolMapItem struct {
	Address string            `json:"address"`
	Labels  map[string]string `json:"labels"`
	Cluster string            `json:"cluster"`
	State   string            `json:"state"`
}

type NodeItem struct {
	NodePoolMapItem
	Name string `json:"-"`
}

func ConvertJsonToNodeItem(jsonStr string) (map[string]NodeItem, error) {
	nodepoolMap := map[string]NodeItem{}

	nodepoolItem, err := ConvertJsonToNodePoolItem(jsonStr)
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

func ConvertJsonToNodePoolItem(jsonStr string) (map[string]NodePoolMapItem, error) {
	nodepoolItem := map[string]NodePoolMapItem{}
	err := json.Unmarshal([]byte(jsonStr), &nodepoolItem)
	if err != nil {
		return nil, err
	}
	return nodepoolItem, nil
}

func ConvertNodePoolItemToJson(nodepoolItem map[string]NodePoolMapItem) ([]byte, error) {
	jsonStr, err := json.Marshal(nodepoolItem)
	if err != nil {
		return nil, err
	}
	return jsonStr, nil
}

// controller task
// TODO： free node need join to  host cluster
// TODO:  check orphan node

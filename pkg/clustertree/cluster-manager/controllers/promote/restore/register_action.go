package restore

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/types"
)

type RestoreItemAction interface {
	// return resource.group
	Resource() []string

	// Execute allows the ItemAction to perform arbitrary logic with the item being backed up,
	// including mutating the item itself prior to backup. The item (unmodified or modified)
	// should be returned, along with an optional slice of ResourceIdentifiers specifying
	// additional related items that should be backed up.
	Execute(obj *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error)

	Revert(fromCluster *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error)
}

func registerRestoreActions() (map[string]RestoreItemAction, error) {
	actionMap := make(map[string]RestoreItemAction, 3)

	err := registerRestoreItemAction(actionMap, newPodRestoreItemAction)
	if err != nil {
		return nil, errors.WithMessage(err, "register pod restore action error")
	}

	err = registerRestoreItemAction(actionMap, newPvRestoreItemAction)
	if err != nil {
		return nil, errors.WithMessage(err, "register pv restore action error")
	}

	err = registerRestoreItemAction(actionMap, newStsDeployRestoreItemAction)
	if err != nil {
		return nil, errors.WithMessage(err, "register sts/deploy restore action error")
	}

	err = registerRestoreItemAction(actionMap, newServiceRestoreItemAction)
	if err != nil {
		return nil, errors.WithMessage(err, "register service restore action error")
	}

	err = registerRestoreItemAction(actionMap, newUniversalRestoreItemAction)
	if err != nil {
		return nil, errors.WithMessage(err, "register universal restore action error")
	}

	return actionMap, nil
}

func registerRestoreItemAction(actionsMap map[string]RestoreItemAction, initializer types.HandlerInitializer) error {
	instance, err := initializer()
	if err != nil {
		return errors.WithMessage(err, "init restore action instance error")
	}

	itemAction, ok := instance.(RestoreItemAction)
	if !ok {
		return errors.Errorf("%T is not a backup item action", instance)
	}
	for _, resource := range itemAction.Resource() {
		actionsMap[resource] = itemAction
	}
	return nil
}

func newPodRestoreItemAction() (interface{}, error) {
	return NewPodAction(), nil
}

func newPvRestoreItemAction() (interface{}, error) {
	return NewPvAction(), nil
}

func newStsDeployRestoreItemAction() (interface{}, error) {
	return NewStsDeployAction(), nil
}

func newServiceRestoreItemAction() (interface{}, error) {
	return NewServiceAction(), nil
}

func newUniversalRestoreItemAction() (interface{}, error) {
	return NewUniversalAction(), nil
}

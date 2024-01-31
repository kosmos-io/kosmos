package detach

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
)

// BackupItemAction is an actor that performs an operation on an individual item being backed up.
type DetachItemAction interface {
	// return resource.group
	Resource() string

	// Execute allows the ItemAction to perform arbitrary logic with the item being backed up,
	// including mutating the item itself prior to backup. The item (unmodified or modified)
	// should be returned, along with an optional slice of ResourceIdentifiers specifying
	// additional related items that should be backed up.
	Execute(obj *unstructured.Unstructured, client client.Dynamic) error
}

func registerDetachActions() (map[string]DetachItemAction, error) {
	actionMap := make(map[string]DetachItemAction, 3)

	if err := registerDetachItemAction(actionMap, newPodDetachItemAction); err != nil {
		return nil, err
	}
	if err := registerDetachItemAction(actionMap, newPvcDetachItemAction); err != nil {
		return nil, err
	}
	return actionMap, nil
}

func registerDetachItemAction(actionsMap map[string]DetachItemAction, initializer requests.HandlerInitializer) error {
	instance, err := initializer()
	if err != nil {
		return errors.WithMessage(err, "init restore action instance error")
	}

	itemAction, ok := instance.(DetachItemAction)
	if !ok {
		return errors.Errorf("%T is not a detach item action", instance)
	}
	actionsMap[itemAction.Resource()] = itemAction
	return nil
}

func newPodDetachItemAction() (interface{}, error) {
	return NewPodAction(), nil
}

func newPvcDetachItemAction() (interface{}, error) {
	return NewPvcAction(), nil
}

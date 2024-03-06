package backup

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/types"
)

// BackupItemAction is an actor that performs an operation on an individual item being backed up.
type BackupItemAction interface {

	// return resource.group
	Resource() string

	// Execute allows the ItemAction to perform arbitrary logic with the item being backed up,
	// including mutating the item itself prior to backup. The item (unmodified or modified)
	// should be returned, along with an optional slice of ResourceIdentifiers specifying
	// additional related items that should be backed up.
	Execute(item runtime.Unstructured, backup *kubernetesBackupper) (runtime.Unstructured, []requests.ResourceIdentifier, error)
}

func registerBackupActions() (map[string]BackupItemAction, error) {
	actionMap := make(map[string]BackupItemAction, 3)

	if err := registerBackupItemAction(actionMap, newPvcBackupItemAction); err != nil {
		return nil, err
	}

	if err := registerBackupItemAction(actionMap, newServiceAccountBackupItemAction); err != nil {
		return nil, err
	}

	return actionMap, nil
}

func registerBackupItemAction(actionsMap map[string]BackupItemAction, initializer types.HandlerInitializer) error {
	instance, err := initializer()
	if err != nil {
		return errors.WithMessage(err, "init backup action instance error")
	}

	itemAction, ok := instance.(BackupItemAction)
	if !ok {
		return errors.Errorf("%T is not a detach item action", instance)
	}
	actionsMap[itemAction.Resource()] = itemAction
	return nil
}

func newPvcBackupItemAction() (interface{}, error) {
	return NewPVCAction(), nil
}

func newServiceAccountBackupItemAction() (interface{}, error) {
	return NewServiceAccountAction(), nil
}

package backup

import (
	"archive/tar"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/discovery"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/types"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/archive"
)

// itemBackupper can back up individual items to a tar writer.
type itemBackupper struct {
	request         *requests.PromoteRequest
	backup          *kubernetesBackupper
	tarWriter       tarWriter
	dynamicFactory  client.DynamicFactory
	discoveryHelper discovery.Helper
	actions         map[string]BackupItemAction
}

type FileForArchive struct {
	FilePath  string
	Header    *tar.Header
	FileBytes []byte
}

func (ib *itemBackupper) backupItem(obj runtime.Unstructured, groupResource schema.GroupResource,
	preferredGVR schema.GroupVersionResource, mustInclude, finalize bool) (bool, []FileForArchive, error) {
	selectedForBackup, files, err := ib.backupItemInternal(obj, groupResource, preferredGVR, mustInclude, finalize)
	if !selectedForBackup || err != nil || len(files) == 0 || finalize {
		return selectedForBackup, files, err
	}

	for _, file := range files {
		if err := ib.tarWriter.WriteHeader(file.Header); err != nil {
			return false, []FileForArchive{}, errors.WithStack(err)
		}

		if _, err := ib.tarWriter.Write(file.FileBytes); err != nil {
			return false, []FileForArchive{}, errors.WithStack(err)
		}
	}

	return true, []FileForArchive{}, nil
}

func (ib *itemBackupper) backupItemInternal(obj runtime.Unstructured, groupResource schema.GroupResource,
	preferredGVR schema.GroupVersionResource, mustInclude, finalize bool) (bool, []FileForArchive, error) {
	var itemFiles []FileForArchive
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return false, itemFiles, err
	}

	namespace := metadata.GetNamespace()
	name := metadata.GetName()

	key := types.ItemKey{
		Resource:  groupResource.String(),
		Namespace: namespace,
		Name:      name,
	}
	if _, exists := ib.request.BackedUpItems[key]; exists {
		klog.Infof("Skipping item %s %s because it's already been backed up.", groupResource.String(), name)
		return true, itemFiles, nil
	}
	ib.request.BackedUpItems[key] = struct{}{}

	klog.Infof("backup item name:%s, resouces: %s, namespace", name, groupResource.String(), namespace)

	if mustInclude {
		klog.Infof("Skipping the exclusion checks for this resource")
	}

	if metadata.GetDeletionTimestamp() != nil {
		klog.Info("Skipping item because it's being deleted.")
		return false, itemFiles, nil
	}

	// capture the version of the object before invoking plugin actions as the plugin may update
	// the group version of the object.
	versionPath := resourceVersion(obj)

	updatedObj, additionalItemFiles, err := ib.executeActions(obj, groupResource, name, namespace)
	if err != nil {
		return false, itemFiles, errors.WithStack(err)
	}

	itemFiles = append(itemFiles, additionalItemFiles...)
	obj = updatedObj
	if metadata, err = meta.Accessor(obj); err != nil {
		return false, itemFiles, errors.WithStack(err)
	}
	// update name and namespace in case they were modified in an action
	name = metadata.GetName()
	namespace = metadata.GetNamespace()

	itemBytes, err := json.Marshal(obj.UnstructuredContent())
	if err != nil {
		return false, itemFiles, errors.WithStack(err)
	}

	//if versionPath == preferredGVR.Version {
	//	// backing up preferred version backup without API Group version - for backward compatibility
	//	log.Infof("Resource %s/%s, version= %s, preferredVersion=%s", groupResource.String(), name, versionPath, preferredGVR.Version)
	//	itemFiles = append(itemFiles, getFileForArchive(namespace, name, groupResource.String(), "", itemBytes))
	//	versionPath = versionPath + constants.PreferredVersionDir
	//}

	itemFiles = append(itemFiles, getFileForArchive(namespace, name, groupResource.String(), versionPath, itemBytes))
	return true, itemFiles, nil
}

func (ib *itemBackupper) executeActions(obj runtime.Unstructured, groupResource schema.GroupResource,
	name, namespace string) (runtime.Unstructured, []FileForArchive, error) {
	var itemFiles []FileForArchive

	if action, ok := ib.actions[groupResource.String()]; ok {
		klog.Info("execute action for %s", groupResource.String())
		updatedItem, additionalItemIdentifiers, err := action.Execute(obj, ib.backup)
		if err != nil {
			return nil, itemFiles, errors.Wrapf(err, "error executing custom action (groupResource=%s, namespace=%s, name=%s)", groupResource.String(), namespace, name)
		}
		u := &unstructured.Unstructured{Object: updatedItem.UnstructuredContent()}
		obj = u

		for _, additionalItem := range additionalItemIdentifiers {
			gvr, resource, err := ib.discoveryHelper.ResourceFor(additionalItem.GroupResource.WithVersion(""))
			if err != nil {
				return nil, itemFiles, err
			}

			client, err := ib.dynamicFactory.ClientForGroupVersionResource(gvr.GroupVersion(), resource, additionalItem.Namespace)
			if err != nil {
				return nil, itemFiles, err
			}

			item, err := client.Get(additionalItem.Name, metav1.GetOptions{})

			if apierrors.IsNotFound(err) {
				klog.Warningf("Additional item was not found in Kubernetes API, can't back it up. groupResouces: %s, namespace: %s, name: %s",
					additionalItem.GroupResource, additionalItem.Namespace, additionalItem.Name)
				continue
			}
			if err != nil {
				return nil, itemFiles, errors.WithStack(err)
			}

			_, additionalItemFiles, err := ib.backupItem(item, gvr.GroupResource(), gvr, true, false)
			if err != nil {
				return nil, itemFiles, err
			}
			itemFiles = append(itemFiles, additionalItemFiles...)
		}
	}

	return obj, itemFiles, nil
}

func getFileForArchive(namespace, name, groupResource, versionPath string, itemBytes []byte) FileForArchive {
	filePath := archive.GetItemFilePath("", groupResource, namespace, name)

	hdr := &tar.Header{
		Name:     filePath,
		Size:     int64(len(itemBytes)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}
	return FileForArchive{FilePath: filePath, Header: hdr, FileBytes: itemBytes}
}

// resourceVersion returns a string representing the object's API Version (e.g.
// v1 if item belongs to apps/v1
func resourceVersion(obj runtime.Unstructured) string {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return gvk.Version
}

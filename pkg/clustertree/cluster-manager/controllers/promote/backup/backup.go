package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	unstructured2 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/constants"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/discovery"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/collections"
)

type kubernetesBackupper struct {
	request         *requests.PromoteRequest
	dynamicFactory  client.DynamicFactory
	discoveryHelper discovery.Helper
	actions         map[string]BackupItemAction
	kubeclient      kubernetes.Interface
}

func NewKubernetesBackupper(request *requests.PromoteRequest) (*kubernetesBackupper, error) {
	actions, err := registerBackupActions()
	if err != nil {
		return nil, err
	}
	dynamicFactory := client.NewDynamicFactory(request.LeafDynamicClient)
	discoveryHelper, err := discovery.NewHelper(request.LeafDiscoveryClient)
	if err != nil {
		return nil, err
	}

	return &kubernetesBackupper{
		request:         request,
		kubeclient:      request.LeafClientSet,
		dynamicFactory:  dynamicFactory,
		discoveryHelper: discoveryHelper,
		actions:         actions,
	}, nil
}

func (kb *kubernetesBackupper) Backup(backupFile io.Writer) error {
	gzippedData := gzip.NewWriter(backupFile)
	defer func(gzippedData *gzip.Writer) {
		_ = gzippedData.Close()
	}(gzippedData)

	tw := tar.NewWriter(gzippedData)
	defer func(tw *tar.Writer) {
		_ = tw.Close()
	}(tw)

	klog.Info("Writing backup version file")
	if err := kb.writeBackupVersion(tw); err != nil {
		return errors.WithStack(err)
	}

	kb.request.ResourceIncludesExcludes = collections.GetScopeResourceIncludesExcludes(kb.discoveryHelper, kb.request.Spec.IncludedNamespaceScopedResources,
		kb.request.Spec.ExcludedNamespaceScopedResources, nil, nil, *kb.request.NamespaceIncludesExcludes)

	// set up a temp dir for the itemCollector to use to temporarily
	// store items as they're scraped from the API.
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return errors.Wrap(err, "error creating temp dir for backup")
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tempDir)

	collector := &itemCollector{
		request:         kb.request,
		discoveryHelper: kb.discoveryHelper,
		dynamicFactory:  kb.dynamicFactory,
		dir:             tempDir,
	}

	items := collector.getAllItems()
	klog.Infof("Collected %d items matching the backup spec from the Kubernetes API (actual number of items backed up may be more or less depending on velero.io/exclude-from-backup annotation, plugins returning additional related items to back up, etc.)", len(items))

	itemBackupper := &itemBackupper{
		request:         kb.request,
		backup:          kb,
		tarWriter:       tw,
		dynamicFactory:  kb.dynamicFactory,
		discoveryHelper: kb.discoveryHelper,
		actions:         kb.actions,
	}

	backedUpGroupResources := map[schema.GroupResource]bool{}

	for _, item := range items {
		klog.Infof("Processing item. resource: %s, namespace: %s, name: %s", item.groupResource.String(), item.namespace, item.name)

		// use an anonymous func so we can defer-close/remove the file
		// as soon as we're done with it
		func() {
			var unstructured unstructured2.Unstructured

			f, err := os.Open(item.path)
			if err != nil {
				klog.Errorf("Error opening file containing item. %v", errors.WithStack(err))
				return
			}
			defer f.Close()
			defer os.Remove(f.Name())

			if err := json.NewDecoder(f).Decode(&unstructured); err != nil {
				klog.Errorf("Error decoding JSON from file. %v", errors.WithStack(err))
				return
			}

			if backedUp := kb.backupItem(item.groupResource, itemBackupper, &unstructured, item.preferredGVR); backedUp {
				backedUpGroupResources[item.groupResource] = true
			}
		}()
	}

	return nil
}

func (kb *kubernetesBackupper) backupItem(gr schema.GroupResource, itemBackupper *itemBackupper, unstructured *unstructured2.Unstructured, preferredGVR schema.GroupVersionResource) bool {
	backedUpItem, _, err := itemBackupper.backupItem(unstructured, gr, preferredGVR, true, false)
	if aggregate, ok := err.(kubeerrs.Aggregate); ok {
		klog.Infof("%d errors encountered backup up item %s", len(aggregate.Errors()), unstructured.GetName())
		// log each error separately so we get error location info in the log, and an
		// accurate count of errors
		for _, err = range aggregate.Errors() {
			klog.Errorf("Error backing up item %s. %v", unstructured.GetName(), err)
		}

		return false
	}

	if err != nil {
		klog.Errorf("Error backing up item %s. %v", unstructured.GetName(), err)
		return false
	}
	return backedUpItem
}

func (kb *kubernetesBackupper) writeBackupVersion(tw *tar.Writer) error {
	versionFile := filepath.Join(constants.MetadataDir, "version")
	versionString := fmt.Sprintf("%s\n", constants.BackupFormatVersion)

	hdr := &tar.Header{
		Name:     versionFile,
		Size:     int64(len(versionString)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return errors.WithStack(err)
	}
	if _, err := tw.Write([]byte(versionString)); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

type tarWriter interface {
	io.Closer
	Write([]byte) (int, error)
	WriteHeader(*tar.Header) error
}

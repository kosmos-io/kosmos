/*
Copyright the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package archive

import (
	"encoding/json"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/constants"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/filesystem"
)

// GetItemFilePath returns an item's file path once extracted from a Velero backup archive.
func GetItemFilePath(rootDir, groupResource, namespace, name string) string {
	return GetVersionedItemFilePath(rootDir, groupResource, namespace, name, "")
}

// GetVersionedItemFilePath returns an item's file path once extracted from a Velero backup archive, with version included.
func GetVersionedItemFilePath(rootDir, groupResource, namespace, name, versionPath string) string {
	path := filepath.Join(rootDir, constants.ResourcesDir, groupResource, versionPath, GetScopeDir(namespace), namespace, name+".json")
	return filepath.ToSlash(path)
}

// GetScopeDir returns NamespaceScopedDir if namespace is present, or ClusterScopedDir if empty
func GetScopeDir(namespace string) string {
	if namespace == "" {
		return constants.ClusterScopedDir
	}
	return constants.NamespaceScopedDir
}

// Unmarshal reads the specified file, unmarshals the JSON contained within it
// and returns an Unstructured object.
func Unmarshal(fs filesystem.Interface, filePath string) (*unstructured.Unstructured, error) {
	var obj unstructured.Unstructured

	bytes, err := fs.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &obj)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

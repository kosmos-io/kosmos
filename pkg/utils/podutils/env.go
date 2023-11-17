// This code is directly lifted from the VIRTUAL-KUBELET
// For reference:
// https://github.com/virtual-kubelet/virtual-kubelet/blob/master/internal/podutils/env.go

package podutils

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	apivalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	// ReasonOptionalConfigMapNotFound is the reason used in events emitted when an optional configmap is not found.
	ReasonOptionalConfigMapNotFound = "OptionalConfigMapNotFound"
	// ReasonOptionalConfigMapKeyNotFound is the reason used in events emitted when an optional configmap key is not found.
	ReasonOptionalConfigMapKeyNotFound = "OptionalConfigMapKeyNotFound"
	// ReasonFailedToReadOptionalConfigMap is the reason used in events emitted when an optional configmap could not be read.
	ReasonFailedToReadOptionalConfigMap = "FailedToReadOptionalConfigMap"

	// ReasonOptionalSecretNotFound is the reason used in events emitted when an optional secret is not found.
	ReasonOptionalSecretNotFound = "OptionalSecretNotFound"
	// ReasonOptionalSecretKeyNotFound is the reason used in events emitted when an optional secret key is not found.
	ReasonOptionalSecretKeyNotFound = "OptionalSecretKeyNotFound"
	// ReasonFailedToReadOptionalSecret is the reason used in events emitted when an optional secret could not be read.
	ReasonFailedToReadOptionalSecret = "FailedToReadOptionalSecret"

	// ReasonMandatoryConfigMapNotFound is the reason used in events emitted when a mandatory configmap is not found.
	ReasonMandatoryConfigMapNotFound = "MandatoryConfigMapNotFound"
	// ReasonMandatoryConfigMapKeyNotFound is the reason used in events emitted when a mandatory configmap key is not found.
	ReasonMandatoryConfigMapKeyNotFound = "MandatoryConfigMapKeyNotFound"
	// ReasonFailedToReadMandatoryConfigMap is the reason used in events emitted when a mandatory configmap could not be read.
	ReasonFailedToReadMandatoryConfigMap = "FailedToReadMandatoryConfigMap"

	// ReasonMandatorySecretNotFound is the reason used in events emitted when a mandatory secret is not found.
	ReasonMandatorySecretNotFound = "MandatorySecretNotFound"
	// ReasonMandatorySecretKeyNotFound is the reason used in events emitted when a mandatory secret key is not found.
	ReasonMandatorySecretKeyNotFound = "MandatorySecretKeyNotFound"
	// ReasonFailedToReadMandatorySecret is the reason used in events emitted when a mandatory secret could not be read.
	ReasonFailedToReadMandatorySecret = "FailedToReadMandatorySecret"

	// ReasonInvalidEnvironmentVariableNames is the reason used in events emitted when a configmap/secret referenced in a ".spec.containers[*].envFrom" field contains invalid environment variable names.
	ReasonInvalidEnvironmentVariableNames = "InvalidEnvironmentVariableNames"
)

var masterServices = sets.NewString("kubernetes")

// PopulateEnvironmentVariables populates the environment of each container (and init container) in the specified pod.
func PopulateEnvironmentVariables(ctx context.Context, pod *corev1.Pod, rm utils.EnvResourceManager) error {
	// Populate each init container's environment.
	for idx := range pod.Spec.InitContainers {
		if err := populateContainerEnvironment(ctx, pod, &pod.Spec.InitContainers[idx], rm); err != nil {
			return err
		}
	}
	// Populate each container's environment.
	for idx := range pod.Spec.Containers {
		if err := populateContainerEnvironment(ctx, pod, &pod.Spec.Containers[idx], rm); err != nil {
			return err
		}
	}
	return nil
}

// populateContainerEnvironment populates the environment of a single container in the specified pod.
func populateContainerEnvironment(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm utils.EnvResourceManager) error {
	// Create an "environment map" based on the value of the specified container's ".envFrom" field.
	tmpEnv, err := makeEnvironmentMapBasedOnEnvFrom(ctx, pod, container, rm)
	if err != nil {
		return err
	}
	// Create the final "environment map" for the container using the ".env" and ".envFrom" field
	// and service environment variables.
	envs, err := makeEnvironmentMap(ctx, pod, container, rm, tmpEnv)
	if err != nil && len(envs) == 0 {
		return err
	}
	// Empty the container's ".envFrom" field and replace its ".env" field with the final, merged environment.
	// Values in "env" (sourced from ".env") will override any values with the same key defined in "envFrom" (sourced from ".envFrom").
	// This is in accordance with what the Kubelet itself does.
	// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubelet/kubelet_pods.go#L557-L558
	container.EnvFrom = []corev1.EnvFromSource{}

	res := make([]corev1.EnvVar, 0, len(tmpEnv))

	for key, val := range tmpEnv {
		res = append(res, corev1.EnvVar{
			Name:  key,
			Value: val,
		})
	}
	res = append(res, envs...)
	container.Env = res

	return nil
}

// getServiceEnvVarMap makes a map[string]string of env vars for services a
// pod in namespace ns should see.
// Based on getServiceEnvVarMap in kubelet_pods.go.
func getServiceEnvVarMap(rm utils.EnvResourceManager, ns string, enableServiceLinks bool) (map[string]string, error) {
	var (
		serviceMap = make(map[string]*corev1.Service)
		m          = make(map[string]string)
	)

	services, err := rm.ListServices()
	if err != nil {
		return nil, err
	}

	// project the services in namespace ns onto the master services
	for i := range services {
		service := services[i]
		// ignore services where ClusterIP is "None" or empty
		if !IsServiceIPSet(service) {
			continue
		}
		serviceName := service.Name

		// We always want to add environment variables for master kubernetes service
		// from the default namespace, even if enableServiceLinks is false.
		// We also add environment variables for other services in the same
		// namespace, if enableServiceLinks is true.
		if service.Namespace == metav1.NamespaceDefault && masterServices.Has(serviceName) {
			if _, exists := serviceMap[serviceName]; !exists {
				serviceMap[serviceName] = service
			}
		} else if service.Namespace == ns && enableServiceLinks {
			serviceMap[serviceName] = service
		}
	}

	mappedServices := make([]*corev1.Service, 0, len(serviceMap))
	for key := range serviceMap {
		mappedServices = append(mappedServices, serviceMap[key])
	}

	for _, e := range FromServices(mappedServices) {
		m[e.Name] = e.Value
	}
	return m, nil
}

// makeEnvironmentMapBasedOnEnvFrom returns a map representing the resolved environment of the specified container after being populated from the entries in the ".envFrom" field.
func makeEnvironmentMapBasedOnEnvFrom(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm utils.EnvResourceManager) (map[string]string, error) {
	// Create a map to hold the resulting environment.
	res := make(map[string]string)
	// Iterate over "envFrom" references in order to populate the environment.
loop:
	for _, envFrom := range container.EnvFrom {
		switch {
		// Handle population from a configmap.
		case envFrom.ConfigMapRef != nil:
			ef := envFrom.ConfigMapRef
			// Check whether the configmap reference is optional.
			// This will control whether we fail when unable to read the configmap.
			optional := ef.Optional != nil && *ef.Optional
			// Try to grab the referenced configmap.
			m, err := rm.GetConfigMap(ef.Name, pod.Namespace)
			if err != nil {
				// We couldn't fetch the configmap.
				// However, if the configmap reference is optional we should not fail.
				if optional {
					if errors.IsNotFound(err) {
						klog.Warningf("configmap %q not found", ef.Name)
					} else {
						klog.Warningf("failed to read configmap %q: %v", ef.Name, err)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the configmap reference is mandatory.
				// Hence, we should return a meaningful error.
				if errors.IsNotFound(err) {
					klog.Warningf("configmap %q not found", ef.Name)
					return nil, fmt.Errorf("configmap %q not found", ef.Name)
				}
				klog.Warningf("failed to read configmap %q", ef.Name)
				return nil, fmt.Errorf("failed to fetch configmap %q: %v", ef.Name, err)
			}
			// At this point we have successfully fetched the target configmap.
			// Iterate over the keys defined in the configmap and populate the environment accordingly.
			// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubelet/kubelet_pods.go#L581-L595
			invalidKeys := make([]string, 0)
		mKeys:
			for key, val := range m.Data {
				// If a prefix has been defined, prepend it to the environment variable's name.
				if len(envFrom.Prefix) > 0 {
					key = envFrom.Prefix + key
				}
				// Make sure that the resulting key is a valid environment variable name.
				// If it isn't, it should be appended to the list of invalid keys and skipped.
				if errMsgs := apivalidation.IsEnvVarName(key); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, key)
					continue mKeys
				}
				// Add the key and its value to the environment.
				res[key] = val
			}
			// Report any invalid keys.
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				klog.Warningf("keys [%s] from configmap %s/%s were skipped since they are invalid as environment variable names", strings.Join(invalidKeys, ", "), m.Namespace, m.Name)
			}
		// Handle population from a secret.
		case envFrom.SecretRef != nil:
			ef := envFrom.SecretRef
			// Check whether the secret reference is optional.
			// This will control whether we fail when unable to read the secret.
			optional := ef.Optional != nil && *ef.Optional
			// Try to grab the referenced secret.
			s, err := rm.GetSecret(ef.Name, pod.Namespace)
			if err != nil {
				// We couldn't fetch the secret.
				// However, if the secret reference is optional we should not fail.
				if optional {
					if errors.IsNotFound(err) {
						klog.Warningf("secret %q not found", ef.Name)
					} else {
						klog.Warningf("failed to read secret %q: %v", ef.Name, err)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the secret reference is mandatory.
				// Hence, we should return a meaningful error.
				if errors.IsNotFound(err) {
					klog.Warningf("secret %q not found", ef.Name)
					return nil, fmt.Errorf("secret %q not found", ef.Name)
				}
				klog.Warningf("failed to read secret %q", ef.Name)
				return nil, fmt.Errorf("failed to fetch secret %q: %v", ef.Name, err)
			}
			// At this point we have successfully fetched the target secret.
			// Iterate over the keys defined in the secret and populate the environment accordingly.
			// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubelet/kubelet_pods.go#L581-L595
			invalidKeys := make([]string, 0)
		sKeys:
			for key, val := range s.Data {
				// If a prefix has been defined, prepend it to the environment variable's name.
				if len(envFrom.Prefix) > 0 {
					key = envFrom.Prefix + key
				}
				// Make sure that the resulting key is a valid environment variable name.
				// If it isn't, it should be appended to the list of invalid keys and skipped.
				if errMsgs := apivalidation.IsEnvVarName(key); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, key)
					continue sKeys
				}
				// Add the key and its value to the environment.
				res[key] = string(val)
			}
			// Report any invalid keys.
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				klog.Warningf("keys [%s] from secret %s/%s were skipped since they are invalid as environment variable names", strings.Join(invalidKeys, ", "), s.Namespace, s.Name)
			}
		}
	}
	// Return the populated environment.
	return res, nil
}

// makeEnvironmentMap returns a map representing the resolved environment of the specified container after being populated from the entries in the ".env" and ".envFrom" field.
func makeEnvironmentMap(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm utils.EnvResourceManager, res map[string]string) ([]corev1.EnvVar, error) {
	// TODO If pod.Spec.EnableServiceLinks is nil then fail as per 1.14 kubelet.
	enableServiceLinks := corev1.DefaultEnableServiceLinks
	if pod.Spec.EnableServiceLinks != nil {
		enableServiceLinks = *pod.Spec.EnableServiceLinks
	}

	// Note that there is a race between Kubelet seeing the pod and kubelet seeing the service.
	// To avoid this users can: (1) wait between starting a service and starting; or (2) detect
	// missing service env var and exit and be restarted; or (3) use DNS instead of env vars
	// and keep trying to resolve the DNS name of the service (recommended).
	svcEnv, err := getServiceEnvVarMap(rm, pod.Namespace, enableServiceLinks)
	if err != nil {
		return nil, err
	}

	// If the variable's Value is set, expand the `$(var)` references to other
	// variables in the .Value field; the sources of variables are the declared
	// variables of the container and the service environment variables.
	// mappingFunc := expansion.MappingFuncFor(res, svcEnv)

	// Iterate over environment variables in order to populate the map.
	var keys []corev1.EnvVar
	for _, env := range container.Env {
		envptr := env
		val, err := getEnvironmentVariableValue(ctx, &envptr, pod, container, rm)
		if err != nil {
			keys = append(keys, env)
		}
		if val != nil {
			res[env.Name] = *val
		}
	}

	// Append service env vars.
	for k, v := range svcEnv {
		if _, present := res[k]; !present {
			res[k] = v
		}
	}

	return keys, nil
}

func getEnvironmentVariableValue(ctx context.Context, env *corev1.EnvVar, pod *corev1.Pod, container *corev1.Container, rm utils.EnvResourceManager) (*string, error) {
	if env.ValueFrom != nil {
		return getEnvironmentVariableValueWithValueFrom(ctx, env, pod, container, rm)
	}
	// Handle values that have been directly provided after expanding variable references.
	return &env.Value, nil
}

func getEnvironmentVariableValueWithValueFrom(ctx context.Context, env *corev1.EnvVar, pod *corev1.Pod, container *corev1.Container, rm utils.EnvResourceManager) (*string, error) {
	// Handle population from a configmap key.
	if env.ValueFrom.ConfigMapKeyRef != nil {
		return getEnvironmentVariableValueWithValueFromConfigMapKeyRef(ctx, env, pod, container, rm)
	}

	// Handle population from a secret key.
	if env.ValueFrom.SecretKeyRef != nil {
		return getEnvironmentVariableValueWithValueFromSecretKeyRef(ctx, env, pod, container, rm)
	}

	// Handle population from a field (downward API).
	if env.ValueFrom.FieldRef != nil {
		return getEnvironmentVariableValueWithValueFromFieldRef(ctx, env, pod, container, rm)
	}
	if env.ValueFrom.ResourceFieldRef != nil {
		// TODO Implement populating resource requests.
		return nil, nil
	}

	klog.Error("Unhandled environment variable with non-nil env.ValueFrom, do not know how to populate")
	return nil, nil
}

func getEnvironmentVariableValueWithValueFromConfigMapKeyRef(ctx context.Context, env *corev1.EnvVar, pod *corev1.Pod, container *corev1.Container, rm utils.EnvResourceManager) (*string, error) {
	// The environment variable must be set from a configmap.
	vf := env.ValueFrom.ConfigMapKeyRef
	// Check whether the key reference is optional.
	// This will control whether we fail when unable to read the requested key.
	optional := vf != nil && vf.Optional != nil && *vf.Optional
	// Try to grab the referenced configmap.
	m, err := rm.GetConfigMap(vf.Name, pod.Namespace)
	if err != nil {
		// We couldn't fetch the configmap.
		// However, if the key reference is optional we should not fail.
		if optional {
			if errors.IsNotFound(err) {
				klog.Warningf("skipping optional envvar %q: configmap %q not found", env.Name, vf.Name)
			} else {
				klog.Warningf("failed to read configmap %q: %v", vf.Name, err)
			}
			// Continue on to the next reference.
			return nil, nil
		}
		// At this point we know the key reference is mandatory.
		// Hence, we should return a meaningful error.
		if errors.IsNotFound(err) {
			klog.Warningf("configmap %q not found", vf.Name)
			return nil, fmt.Errorf("configmap %q not found", vf.Name)
		}
		klog.Warningf("failed to read configmap %q", vf.Name)
		return nil, fmt.Errorf("failed to read configmap %q: %v", vf.Name, err)
	}
	// At this point we have successfully fetched the target configmap.
	// We must now try to grab the requested key.
	var (
		keyExists bool
		keyValue  string
	)
	if keyValue, keyExists = m.Data[vf.Key]; !keyExists {
		// The requested key does not exist.
		// However, we should not fail if the key reference is optional.
		if optional {
			// Continue on to the next reference.
			klog.Warningf("skipping optional envvar %q: key %q does not exist in configmap %q", env.Name, vf.Key, vf.Name)
			return nil, nil
		}
		// At this point we know the key reference is mandatory.
		// Hence, we should fail.
		klog.Warningf("key %q does not exist in configmap %q", vf.Key, vf.Name)
		return nil, fmt.Errorf("configmap %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
	}
	// Populate the environment variable and continue on to the next reference.
	return &keyValue, nil
}

func getEnvironmentVariableValueWithValueFromSecretKeyRef(ctx context.Context, env *corev1.EnvVar, pod *corev1.Pod, container *corev1.Container, rm utils.EnvResourceManager) (*string, error) {
	vf := env.ValueFrom.SecretKeyRef
	// Check whether the key reference is optional.
	// This will control whether we fail when unable to read the requested key.
	optional := vf != nil && vf.Optional != nil && *vf.Optional
	// Try to grab the referenced secret.
	s, err := rm.GetSecret(vf.Name, pod.Namespace)
	if err != nil {
		// We couldn't fetch the secret.
		// However, if the key reference is optional we should not fail.
		if optional {
			if errors.IsNotFound(err) {
				klog.Warningf("skipping optional envvar %q: secret %q not found", env.Name, vf.Name)
			} else {
				klog.Warningf("failed to read secret %q: %v", vf.Name, err)
				klog.Warningf("skipping optional envvar %q: failed to read secret %q", env.Name, vf.Name)
			}
			// Continue on to the next reference.
			return nil, nil
		}
		// At this point we know the key reference is mandatory.
		// Hence, we should return a meaningful error.
		if errors.IsNotFound(err) {
			klog.Warningf("secret %q not found", vf.Name)
			return nil, fmt.Errorf("secret %q not found", vf.Name)
		}
		klog.Warningf("failed to read secret %q", vf.Name)
		return nil, fmt.Errorf("failed to read secret %q: %v", vf.Name, err)
	}
	// At this point we have successfully fetched the target secret.
	// We must now try to grab the requested key.
	var (
		keyExists bool
		keyValue  []byte
	)
	if keyValue, keyExists = s.Data[vf.Key]; !keyExists {
		// The requested key does not exist.
		// However, we should not fail if the key reference is optional.
		if optional {
			// Continue on to the next reference.
			klog.Warningf("skipping optional envvar %q: key %q does not exist in secret %q", env.Name, vf.Key, vf.Name)
			return nil, nil
		}
		// At this point we know the key reference is mandatory.
		// Hence, we should fail.
		klog.Warningf("key %q does not exist in secret %q", vf.Key, vf.Name)
		return nil, fmt.Errorf("secret %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
	}
	// Populate the environment variable and continue on to the next reference.
	ret := string(keyValue)
	return &ret, nil
}

// Handle population from a field (downward API).
func getEnvironmentVariableValueWithValueFromFieldRef(ctx context.Context, env *corev1.EnvVar, pod *corev1.Pod, container *corev1.Container, rm utils.EnvResourceManager) (*string, error) {
	// https://github.com/virtual-kubelet/virtual-kubelet/issues/123
	vf := env.ValueFrom.FieldRef

	runtimeVal, err := podFieldSelectorRuntimeValue(vf, pod)
	if err != nil {
		return nil, err
	}

	return &runtimeVal, nil
}

// podFieldSelectorRuntimeValue returns the runtime value of the given
// selector for a pod.
func podFieldSelectorRuntimeValue(fs *corev1.ObjectFieldSelector, pod *corev1.Pod) (string, error) {
	internalFieldPath, _, err := ConvertDownwardAPIFieldLabel(fs.APIVersion, fs.FieldPath, "")
	if err != nil {
		return "", err
	}
	switch internalFieldPath {
	case "spec.nodeName":
		return pod.Spec.NodeName, nil
	case "spec.serviceAccountName":
		return pod.Spec.ServiceAccountName, nil
	}
	return ExtractFieldPathAsString(pod, internalFieldPath)
}

// ExtractFieldPathAsString extracts the field from the given object
// and returns it as a string.  The object must be a pointer to an
// API type.
func ExtractFieldPathAsString(obj interface{}, fieldPath string) (string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}

	if path, subscript, ok := SplitMaybeSubscriptedPath(fieldPath); ok {
		switch path {
		case "metadata.annotations":
			if errs := apivalidation.IsQualifiedName(strings.ToLower(subscript)); len(errs) != 0 {
				return "", fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}
			return accessor.GetAnnotations()[subscript], nil
		case "metadata.labels":
			if errs := apivalidation.IsQualifiedName(subscript); len(errs) != 0 {
				return "", fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}
			return accessor.GetLabels()[subscript], nil
		default:
			return "", fmt.Errorf("fieldPath %q does not support subscript", fieldPath)
		}
	}

	switch fieldPath {
	case "metadata.annotations":
		return FormatMap(accessor.GetAnnotations()), nil
	case "metadata.labels":
		return FormatMap(accessor.GetLabels()), nil
	case "metadata.name":
		return accessor.GetName(), nil
	case "metadata.namespace":
		return accessor.GetNamespace(), nil
	case "metadata.uid":
		return string(accessor.GetUID()), nil
	}

	return "", fmt.Errorf("unsupported fieldPath: %v", fieldPath)
}

// FormatMap formats map[string]string to a string.
func FormatMap(m map[string]string) (fmtStr string) {
	// output with keys in sorted order to provide stable output
	keys := sets.NewString()
	for key := range m {
		keys.Insert(key)
	}
	for _, key := range keys.List() {
		fmtStr += fmt.Sprintf("%v=%q\n", key, m[key])
	}
	fmtStr = strings.TrimSuffix(fmtStr, "\n")

	return
}

// SplitMaybeSubscriptedPath checks whether the specified fieldPath is
// subscripted, and
//   - if yes, this function splits the fieldPath into path and subscript, and
//     returns (path, subscript, true).
//   - if no, this function returns (fieldPath, "", false).
//
// Example inputs and outputs:
//   - "metadata.annotations['myKey']" --> ("metadata.annotations", "myKey", true)
//   - "metadata.annotations['a[b]c']" --> ("metadata.annotations", "a[b]c", true)
//   - "metadata.labels[”]"           --> ("metadata.labels", "", true)
//   - "metadata.labels"               --> ("metadata.labels", "", false)
func SplitMaybeSubscriptedPath(fieldPath string) (string, string, bool) {
	if !strings.HasSuffix(fieldPath, "']") {
		return fieldPath, "", false
	}
	s := strings.TrimSuffix(fieldPath, "']")
	parts := strings.SplitN(s, "['", 2)
	if len(parts) < 2 {
		return fieldPath, "", false
	}
	if len(parts[0]) == 0 {
		return fieldPath, "", false
	}
	return parts[0], parts[1], true
}

// ConvertDownwardAPIFieldLabel converts the specified downward API field label
// and its value in the pod of the specified version to the internal version,
// and returns the converted label and value. This function returns an error if
// the conversion fails.
func ConvertDownwardAPIFieldLabel(version, label, value string) (string, string, error) {
	if version != "v1" {
		return "", "", fmt.Errorf("unsupported pod version: %s", version)
	}

	if path, _, ok := SplitMaybeSubscriptedPath(label); ok {
		switch path {
		case "metadata.annotations", "metadata.labels":
			return label, value, nil
		default:
			return "", "", fmt.Errorf("field label does not support subscript: %s", label)
		}
	}

	switch label {
	case "metadata.annotations",
		"metadata.labels",
		"metadata.name",
		"metadata.namespace",
		"metadata.uid",
		"spec.nodeName",
		"spec.restartPolicy",
		"spec.serviceAccountName",
		"spec.schedulerName",
		"status.phase",
		"status.hostIP",
		"status.podIP",
		"status.podIPs":
		return label, value, nil
	// This is for backwards compatibility with old v1 clients which send spec.host
	case "spec.host":
		return "spec.nodeName", value, nil
	default:
		return "", "", fmt.Errorf("field label not supported: %s", label)
	}
}

// this function aims to check if the service's ClusterIP is set or not
// the objective is not to perform validation here
func IsServiceIPSet(service *corev1.Service) bool {
	return service.Spec.ClusterIP != corev1.ClusterIPNone && service.Spec.ClusterIP != ""
}

// provided as an argument.
func FromServices(services []*corev1.Service) []corev1.EnvVar {
	var result []corev1.EnvVar
	for i := range services {
		service := services[i]

		// ignore services where ClusterIP is "None" or empty
		// the services passed to this method should be pre-filtered
		// only services that have the cluster IP set should be included here
		if !IsServiceIPSet(service) {
			continue
		}

		// Host
		name := makeEnvVariableName(service.Name) + "_SERVICE_HOST"
		result = append(result, corev1.EnvVar{Name: name, Value: service.Spec.ClusterIP})
		// First port - give it the backwards-compatible name
		name = makeEnvVariableName(service.Name) + "_SERVICE_PORT"
		result = append(result, corev1.EnvVar{Name: name, Value: strconv.Itoa(int(service.Spec.Ports[0].Port))})
		// All named ports (only the first may be unnamed, checked in validation)
		for i := range service.Spec.Ports {
			sp := &service.Spec.Ports[i]
			if sp.Name != "" {
				pn := name + "_" + makeEnvVariableName(sp.Name)
				result = append(result, corev1.EnvVar{Name: pn, Value: strconv.Itoa(int(sp.Port))})
			}
		}
		// Docker-compatible vars.
		result = append(result, makeLinkVariables(service)...)
	}
	return result
}

func makeEnvVariableName(str string) string {
	// TODO: If we simplify to "all names are DNS1123Subdomains" this
	// will need two tweaks:
	//   1) Handle leading digits
	//   2) Handle dots
	return strings.ToUpper(strings.Replace(str, "-", "_", -1))
}

func makeLinkVariables(service *corev1.Service) []corev1.EnvVar {
	prefix := makeEnvVariableName(service.Name)
	all := []corev1.EnvVar{}
	for i := range service.Spec.Ports {
		sp := &service.Spec.Ports[i]

		protocol := string(corev1.ProtocolTCP)
		if sp.Protocol != "" {
			protocol = string(sp.Protocol)
		}

		hostPort := net.JoinHostPort(service.Spec.ClusterIP, strconv.Itoa(int(sp.Port)))

		if i == 0 {
			// Docker special-cases the first port.
			all = append(all, corev1.EnvVar{
				Name:  prefix + "_PORT",
				Value: fmt.Sprintf("%s://%s", strings.ToLower(protocol), hostPort),
			})
		}
		portPrefix := fmt.Sprintf("%s_PORT_%d_%s", prefix, sp.Port, strings.ToUpper(protocol))
		all = append(all, []corev1.EnvVar{
			{
				Name:  portPrefix,
				Value: fmt.Sprintf("%s://%s", strings.ToLower(protocol), hostPort),
			},
			{
				Name:  portPrefix + "_PROTO",
				Value: strings.ToLower(protocol),
			},
			{
				Name:  portPrefix + "_PORT",
				Value: strconv.Itoa(int(sp.Port)),
			},
			{
				Name:  portPrefix + "_ADDR",
				Value: service.Spec.ClusterIP,
			},
		}...)
	}
	return all
}

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
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils/expansion"
	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils/manager"
)

const (
	ReasonOptionalConfigMapNotFound     = "OptionalConfigMapNotFound"
	ReasonOptionalConfigMapKeyNotFound  = "OptionalConfigMapKeyNotFound"
	ReasonFailedToReadOptionalConfigMap = "FailedToReadOptionalConfigMap"

	ReasonOptionalSecretNotFound     = "OptionalSecretNotFound"
	ReasonOptionalSecretKeyNotFound  = "OptionalSecretKeyNotFound"
	ReasonFailedToReadOptionalSecret = "FailedToReadOptionalSecret"

	ReasonMandatoryConfigMapNotFound     = "MandatoryConfigMapNotFound"
	ReasonMandatoryConfigMapKeyNotFound  = "MandatoryConfigMapKeyNotFound"
	ReasonFailedToReadMandatoryConfigMap = "FailedToReadMandatoryConfigMap"

	ReasonMandatorySecretNotFound     = "MandatorySecretNotFound"
	ReasonMandatorySecretKeyNotFound  = "MandatorySecretKeyNotFound"
	ReasonFailedToReadMandatorySecret = "FailedToReadMandatorySecret"

	ReasonInvalidEnvironmentVariableNames = "InvalidEnvironmentVariableNames"
)

var masterServices = sets.NewString("kubernetes")

func PopulateEnvironmentVariables(ctx context.Context, pod *corev1.Pod, rm *manager.ResourceManager, recorder record.EventRecorder) error {
	for idx := range pod.Spec.InitContainers {
		if err := populateContainerEnvironment(ctx, pod, &pod.Spec.InitContainers[idx], rm, recorder); err != nil {
			return err
		}
	}
	for idx := range pod.Spec.Containers {
		if err := populateContainerEnvironment(ctx, pod, &pod.Spec.Containers[idx], rm, recorder); err != nil {
			return err
		}
	}
	return nil
}

func populateContainerEnvironment(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) error {
	tmpEnv, err := makeEnvironmentMapBasedOnEnvFrom(ctx, pod, container, rm, recorder)
	if err != nil {
		return err
	}

	err = makeEnvironmentMap(ctx, pod, container, rm, recorder, tmpEnv)
	if err != nil {
		return err
	}

	container.EnvFrom = []corev1.EnvFromSource{}

	res := make([]corev1.EnvVar, 0, len(tmpEnv))

	for key, val := range tmpEnv {
		res = append(res, corev1.EnvVar{
			Name:  key,
			Value: val,
		})
	}
	container.Env = res

	return nil
}

func getServiceEnvVarMap(rm *manager.ResourceManager, ns string, enableServiceLinks bool) (map[string]string, error) {
	var (
		serviceMap = make(map[string]*corev1.Service)
		m          = make(map[string]string)
	)

	services, err := rm.ListServices()
	if err != nil {
		return nil, err
	}

	for i := range services {
		service := services[i]

		if !IsServiceIPSet(service) {
			continue
		}
		serviceName := service.Name

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

func makeEnvironmentMapBasedOnEnvFrom(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) (map[string]string, error) {
	res := make(map[string]string)
loop:
	for _, envFrom := range container.EnvFrom {
		switch {
		case envFrom.ConfigMapRef != nil:
			ef := envFrom.ConfigMapRef
			optional := ef.Optional != nil && *ef.Optional
			m, err := rm.GetConfigMap(ef.Name, pod.Namespace)
			if err != nil {
				if optional {
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapNotFound, "configmap %q not found", ef.Name)
					} else {
						klog.Warningf("failed to read configmap %q: %v", ef.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalConfigMap, "failed to read configmap %q", ef.Name)
					}
					continue loop
				}
				if errors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapNotFound, "configmap %q not found", ef.Name)
					return nil, fmt.Errorf("configmap %q not found", ef.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatoryConfigMap, "failed to read configmap %q", ef.Name)
				return nil, fmt.Errorf("failed to fetch configmap %q: %v", ef.Name, err)
			}
			invalidKeys := make([]string, 0)
		mKeys:
			for key, val := range m.Data {
				if len(envFrom.Prefix) > 0 {
					key = envFrom.Prefix + key
				}
				if errMsgs := apivalidation.IsEnvVarName(key); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, key)
					continue mKeys
				}
				res[key] = val
			}
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonInvalidEnvironmentVariableNames, "keys [%s] from configmap %s/%s were skipped since they are invalid as environment variable names", strings.Join(invalidKeys, ", "), m.Namespace, m.Name)
			}

		case envFrom.SecretRef != nil:
			ef := envFrom.SecretRef
			optional := ef.Optional != nil && *ef.Optional
			s, err := rm.GetSecret(ef.Name, pod.Namespace)
			if err != nil {
				if optional {
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretNotFound, "secret %q not found", ef.Name)
					} else {
						klog.Warningf("failed to read secret %q: %v", ef.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalSecret, "failed to read secret %q", ef.Name)
					}
					continue loop
				}
				if errors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretNotFound, "secret %q not found", ef.Name)
					return nil, fmt.Errorf("secret %q not found", ef.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatorySecret, "failed to read secret %q", ef.Name)
				return nil, fmt.Errorf("failed to fetch secret %q: %v", ef.Name, err)
			}
			invalidKeys := make([]string, 0)
		sKeys:
			for key, val := range s.Data {
				if len(envFrom.Prefix) > 0 {
					key = envFrom.Prefix + key
				}
				if errMsgs := apivalidation.IsEnvVarName(key); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, key)
					continue sKeys
				}
				res[key] = string(val)
			}
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonInvalidEnvironmentVariableNames, "keys [%s] from secret %s/%s were skipped since they are invalid as environment variable names", strings.Join(invalidKeys, ", "), s.Namespace, s.Name)
			}
		}
	}

	return res, nil
}

func makeEnvironmentMap(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder, res map[string]string) error {
	enableServiceLinks := corev1.DefaultEnableServiceLinks
	if pod.Spec.EnableServiceLinks != nil {
		enableServiceLinks = *pod.Spec.EnableServiceLinks
	}

	svcEnv, err := getServiceEnvVarMap(rm, pod.Namespace, enableServiceLinks)
	if err != nil {
		return err
	}

	mappingFunc := expansion.MappingFuncFor(res, svcEnv)

	for _, env := range container.Env {
		envPt := env
		val, err := getEnvironmentVariableValue(ctx, &envPt, mappingFunc, pod, container, rm, recorder)
		if err != nil {
			return err
		}
		if val != nil {
			res[env.Name] = *val
		}
	}

	for k, v := range svcEnv {
		if _, present := res[k]; !present {
			res[k] = v
		}
	}

	return nil
}

func getEnvironmentVariableValue(ctx context.Context, env *corev1.EnvVar, mappingFunc func(string) string, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) (*string, error) {
	if env.ValueFrom != nil {
		return getEnvironmentVariableValueWithValueFrom(ctx, env, mappingFunc, pod, container, rm, recorder)
	}
	ret := expansion.Expand(env.Value, mappingFunc)
	return &ret, nil
}

func getEnvironmentVariableValueWithValueFrom(ctx context.Context, env *corev1.EnvVar, mappingFunc func(string) string, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) (*string, error) {
	if env.ValueFrom.ConfigMapKeyRef != nil {
		return getEnvironmentVariableValueWithValueFromConfigMapKeyRef(ctx, env, mappingFunc, pod, container, rm, recorder)
	}

	if env.ValueFrom.SecretKeyRef != nil {
		return getEnvironmentVariableValueWithValueFromSecretKeyRef(ctx, env, mappingFunc, pod, container, rm, recorder)
	}

	if env.ValueFrom.FieldRef != nil {
		return getEnvironmentVariableValueWithValueFromFieldRef(ctx, env, mappingFunc, pod, container, rm, recorder)
	}
	if env.ValueFrom.ResourceFieldRef != nil {
		return nil, nil
	}

	klog.Error("Unhandled environment variable with non-nil env.ValueFrom, do not know how to populate")
	return nil, nil
}

func getEnvironmentVariableValueWithValueFromConfigMapKeyRef(ctx context.Context, env *corev1.EnvVar, mappingFunc func(string) string, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) (*string, error) {
	vf := env.ValueFrom.ConfigMapKeyRef

	optional := vf != nil && vf.Optional != nil && *vf.Optional

	m, err := rm.GetConfigMap(vf.Name, pod.Namespace)
	if err != nil {
		if optional {
			if errors.IsNotFound(err) {
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapNotFound, "skipping optional envvar %q: configmap %q not found", env.Name, vf.Name)
			} else {
				klog.Warningf("failed to read configmap %q: %v", vf.Name, err)
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalConfigMap, "skipping optional envvar %q: failed to read configmap %q", env.Name, vf.Name)
			}
			return nil, nil
		}
		if errors.IsNotFound(err) {
			recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapNotFound, "configmap %q not found", vf.Name)
			return nil, fmt.Errorf("configmap %q not found", vf.Name)
		}
		recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatoryConfigMap, "failed to read configmap %q", vf.Name)
		return nil, fmt.Errorf("failed to read configmap %q: %v", vf.Name, err)
	}
	var (
		keyExists bool
		keyValue  string
	)
	if keyValue, keyExists = m.Data[vf.Key]; !keyExists {
		if optional {
			recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapKeyNotFound, "skipping optional envvar %q: key %q does not exist in configmap %q", env.Name, vf.Key, vf.Name)
			return nil, nil
		}
		recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapKeyNotFound, "key %q does not exist in configmap %q", vf.Key, vf.Name)
		return nil, fmt.Errorf("configmap %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
	}
	return &keyValue, nil
}

func getEnvironmentVariableValueWithValueFromSecretKeyRef(ctx context.Context, env *corev1.EnvVar, mappingFunc func(string) string, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) (*string, error) {
	vf := env.ValueFrom.SecretKeyRef
	optional := vf != nil && vf.Optional != nil && *vf.Optional
	s, err := rm.GetSecret(vf.Name, pod.Namespace)
	if err != nil {
		if optional {
			if errors.IsNotFound(err) {
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretNotFound, "skipping optional envvar %q: secret %q not found", env.Name, vf.Name)
			} else {
				klog.Warningf("failed to read secret %q: %v", vf.Name, err)
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalSecret, "skipping optional envvar %q: failed to read secret %q", env.Name, vf.Name)
			}
			return nil, nil
		}
		if errors.IsNotFound(err) {
			recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretNotFound, "secret %q not found", vf.Name)
			return nil, fmt.Errorf("secret %q not found", vf.Name)
		}
		recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatorySecret, "failed to read secret %q", vf.Name)
		return nil, fmt.Errorf("failed to read secret %q: %v", vf.Name, err)
	}
	var (
		keyExists bool
		keyValue  []byte
	)
	if keyValue, keyExists = s.Data[vf.Key]; !keyExists {
		if optional {
			recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretKeyNotFound, "skipping optional envvar %q: key %q does not exist in secret %q", env.Name, vf.Key, vf.Name)
			return nil, nil
		}
		recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretKeyNotFound, "key %q does not exist in secret %q", vf.Key, vf.Name)
		return nil, fmt.Errorf("secret %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
	}
	ret := string(keyValue)
	return &ret, nil
}

func getEnvironmentVariableValueWithValueFromFieldRef(ctx context.Context, env *corev1.EnvVar, mappingFunc func(string) string, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) (*string, error) {
	vf := env.ValueFrom.FieldRef

	runtimeVal, err := podFieldSelectorRuntimeValue(vf, pod)
	if err != nil {
		return nil, err
	}

	return &runtimeVal, nil
}

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

func FormatMap(m map[string]string) (fmtStr string) {
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
	case "spec.host":
		return "spec.nodeName", value, nil
	default:
		return "", "", fmt.Errorf("field label not supported: %s", label)
	}
}

func IsServiceIPSet(service *corev1.Service) bool {
	return service.Spec.ClusterIP != corev1.ClusterIPNone && service.Spec.ClusterIP != ""
}

func FromServices(services []*corev1.Service) []corev1.EnvVar {
	var result []corev1.EnvVar
	for i := range services {
		service := services[i]

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

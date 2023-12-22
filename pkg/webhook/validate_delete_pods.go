package webhook

import (
	"context"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

type PodValidator struct {
	client.Client
	decoder *admission.Decoder
	Options PodValidatorOptions
}

type PodValidatorOptions struct {
	UsernamesNeedToPrevent []string
}

var _ admission.Handler = &PodValidator{}
var _ admission.DecoderInjector = &PodValidator{}

func (v *PodValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Delete {
		return admission.Allowed("")
	}

	pod := &corev1.Pod{}
	err := v.decoder.DecodeRaw(req.OldObject, pod)
	if err != nil {
		klog.Warningf("Decode oldObject error: %v, skip", err)
		return admission.Allowed("")
	}

	if pod.Spec.NodeName == "" {
		klog.V(4).Infof("Pod %s's nodeName is empty, skip", err)
		return admission.Allowed("")
	}

	node := &corev1.Node{}
	if err = v.Client.Get(ctx, types.NamespacedName{
		Name: pod.Spec.NodeName,
	}, node); err != nil {
		klog.V(4).Infof("Failed to get pod %s/%s's node, nodeName: %s, error: %v", pod.Namespace, pod.Name, pod.Spec.NodeName, err)
		return admission.Allowed("")
	}

	if utils.IsKosmosNode(node) && utils.IsNotReady(node) && v.needToPrevent(req.UserInfo.Username) {
		klog.Infof("Kosmos prevents pod deletion, name: %s, ns: %s", pod.Name, pod.Namespace)
		return admission.Denied("Deleting pods of notReady kosmos nodes is not allowed.")
	}

	return admission.Allowed("")
}

func (v *PodValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

func (v *PodValidator) needToPrevent(username string) bool {
	return slices.Contains(v.Options.UsernamesNeedToPrevent, username)
}

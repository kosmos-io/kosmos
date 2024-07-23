package virtualcluster_plugin_controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/plugins"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type VirtualClusterPluginController struct {
	client.Client
	RootClientSet kubernetes.Interface
	KosmosClient  versioned.Interface
	EventRecorder record.EventRecorder
}

func (vcpc *VirtualClusterPluginController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ virtual-cluster-plugin-controller start to reconcile %s ============", request.NamespacedName)
	defer klog.V(4).Infof("============ virtual-cluster-plugin-controller finish to reconcile %s ============", request.NamespacedName)

	var vc v1alpha1.VirtualCluster
	if err := vcpc.Get(ctx, request.NamespacedName, &vc); err != nil {
		klog.V(4).Infof("virtual-cluster-plugin-controller: can not found %s", request.NamespacedName)
		return reconcile.Result{}, nil
	}

	if vc.Status.Phase != v1alpha1.AllNodeReady {
		// wait virtual cluster all node ready
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	for _, plugin := range vc.Spec.PluginSet.Enabled {
		vcp, err := vcpc.KosmosClient.KosmosV1alpha1().VirtualClusterPlugins(vc.Namespace).Get(ctx, plugin.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("virtual-cluster-plugin-controller: unable to find plugin %s, skip install", plugin.Name)
			continue
		}

		if vcp.Spec.PluginSources.Yaml != (v1alpha1.Yaml{}) {
			err = vcpc.createPluginExecutorJobByYaml(ctx, vcp)
			if err != nil {
				klog.Errorf("virtual-cluster-plugin-controller: unable to create plugin executor job, skip install %s", plugin.Name)
				continue
			} else {
				klog.Infof("virtual-cluster-plugin-controller: plugin %s executor job create success", plugin.Name)
			}
		} else {
			err = vcpc.createPluginExecutorJobByHelm(ctx, vcp)
			if err != nil {
				klog.Errorf("virtual-cluster-plugin-controller: unable to create plugin executor job, skip install %s", plugin.Name)
				continue
			} else {
				klog.Infof("virtual-cluster-plugin-controller: plugin %s executor job create success", plugin.Name)
			}
		}
	}

	return reconcile.Result{}, nil
}

// Function to create a plugin executor job
func (vcpc *VirtualClusterPluginController) createPluginExecutorJobByYaml(ctx context.Context, vcp *v1alpha1.VirtualClusterPlugin) error {
	var pluginYamlContent string
	switch vcp.Name {
	case "node-local-dns":
		nodeLocalDNS, err := util.ParseTemplate(plugins.NodeLocalDNSPlugin, struct {
			Domain, KubeDNS, LocalDNS, ClusterDNS string
			ImageRepository, Version              string
		}{
			Domain:          vcp.Spec.PluginSources.Yaml.Domain,
			KubeDNS:         vcp.Spec.PluginSources.Yaml.KubeDNS,
			LocalDNS:        vcp.Spec.PluginSources.Yaml.LocalDNS,
			ClusterDNS:      vcp.Spec.PluginSources.Yaml.ClusterDNS,
			ImageRepository: vcp.Spec.PluginSources.Yaml.ImageRepository,
			Version:         vcp.Spec.PluginSources.Yaml.Version,
		})
		if err != nil {
			return fmt.Errorf("error when parsing virtual cluster plugin node-local-dns: %w", err)
		}
		pluginYamlContent = nodeLocalDNS
	}

	pluginJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("plugin-executor-job-%s", vcp.Name),
			Namespace: vcp.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name: "plugin-executor",
							// ToDo Use unified image, requirements: contains kubectl & helm binary exec files
							Image:   vcp.Spec.PluginSources.Yaml.ImageRepository + "/kubectl:v1.25.7",
							Command: []string{"sh", "-c"},
							Args: []string{
								fmt.Sprintf("echo '%s' | kubectl apply -f -", pluginYamlContent),
							},
						},
					},
				},
			},
		},
	}

	_, err := vcpc.RootClientSet.BatchV1().Jobs(vcp.Namespace).Create(ctx, pluginJob, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error when creating virtual cluster plugin %s, err: %w", pluginJob.Name, err)
	}

	return nil
}

func (vcpc *VirtualClusterPluginController) createPluginExecutorJobByHelm(ctx context.Context, vcp *v1alpha1.VirtualClusterPlugin) error {
	pluginJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("plugin-executor-job-%s", vcp.Name),
			Namespace: vcp.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name: "plugin-executor",
							// ToDo Use unified image, requirements: contains kubectl & helm binary exec files
							Image:   "",
							Command: []string{"sh", "-c"},
							Args: []string{
								fmt.Sprintf("helm install %s /mnt/helm-charts/%s", vcp.Name, vcp.Name),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plugin-charts",
									MountPath: "/mnt/plugin-charts",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "plugin-charts",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									// local charts path
									Path: vcp.Spec.PluginSources.Chart.Storage.HostPath.Path,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := vcpc.RootClientSet.BatchV1().Jobs(vcp.Namespace).Create(ctx, pluginJob, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error when creating virtual cluster plugin %s, err: %w", pluginJob.Name, err)
	}

	return nil
}

func (vcpc *VirtualClusterPluginController) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.PluginControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&v1alpha1.VirtualCluster{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return true
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return true
			},
		})).
		Complete(vcpc)
}

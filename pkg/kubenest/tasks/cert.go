package tasks

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/util/cert"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewCertTask() workflow.Task {
	return workflow.Task{
		Name:        "Certs",
		Run:         runCerts,
		Skip:        skipCerts,
		RunSubTasks: true,
		Tasks:       newCertSubTasks(),
	}
}

func runCerts(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("certs task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[certs] Running certs task", "virtual cluster", klog.KObj(data))
	return nil
}

func skipCerts(d workflow.RunData) (bool, error) {
	data, ok := d.(InitData)
	if !ok {
		return false, errors.New("certs task invoked with an invalid data struct")
	}

	secretName := fmt.Sprintf("%s-%s", data.GetName(), "cert")
	secret, err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return false, nil
	}

	if err := data.LoadCertFromSecret(secret); err != nil {
		return false, err
	}

	klog.V(4).InfoS("[certs] Successfully loaded certs form secret", "secret", secret.Name, "virtual cluster", klog.KObj(data))
	klog.V(2).InfoS("[certs] Skip certs task, found previous certificates in secret", "virtual cluster", klog.KObj(data))
	return true, nil
}

func newCertSubTasks() []workflow.Task {
	var subTasks []workflow.Task
	caCert := map[string]*cert.CertConfig{}

	for _, cert := range cert.GetDefaultCertList() {
		var task workflow.Task
		if cert.CAName == "" {
			task = workflow.Task{Name: cert.Name, Run: runCATask(cert)}
			caCert[cert.Name] = cert
		} else {
			task = workflow.Task{Name: cert.Name, Run: runCertTask(cert, caCert[cert.CAName])}
		}

		subTasks = append(subTasks, task)
	}

	return subTasks
}

func runCertTask(cc, caCert *cert.CertConfig) func(d workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return fmt.Errorf("certs task invoked with an invalid data struct")
		}

		if caCert == nil {
			return fmt.Errorf("unexpected empty ca cert for %s", cc.Name)
		}

		if cc.CAName != caCert.Name {
			return fmt.Errorf("expected CAname for %s, but was %s", cc.CAName, cc.Name)
		}

		if err := mutateCertConfig(data, cc); err != nil {
			return fmt.Errorf("error when mutate cert altNames for %s, err: %w", cc.Name, err)
		}

		caCert := data.GetCert(cc.CAName)
		cert, err := cert.CreateCertAndKeyFilesWithCA(cc, caCert.CertData(), caCert.KeyData())
		if err != nil {
			return err
		}

		data.AddCert(cert)

		klog.V(2).InfoS("[certs] Successfully generated certificate", "certName", cc.Name, "caName", cc.CAName)
		return nil
	}
}

func runCATask(kc *cert.CertConfig) func(d workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return errors.New("certs task invoked with an invalid data struct")
		}

		if kc.CAName != "" {
			return fmt.Errorf("this function should only be used for CAs, but cert %s has CA %s", kc.Name, kc.CAName)
		}
		klog.V(4).InfoS("[certs] Creating a new certificate authority", "certName", kc.Name)

		cert, err := cert.NewCertificateAuthority(kc)
		if err != nil {
			return err
		}

		klog.V(2).InfoS("[certs] Successfully generated ca certificate", "certName", kc.Name)

		data.AddCert(cert)
		return nil
	}
}

func mutateCertConfig(data InitData, cc *cert.CertConfig) error {
	if cc.AltNamesMutatorFunc != nil {
		err := cc.AltNamesMutatorFunc(&cert.AltNamesMutatorConfig{
			Name:             data.GetName(),
			Namespace:        data.GetNamespace(),
			ControlplaneAddr: data.ControlplaneAddress(),
			ClusterIps:       data.ServiceClusterIp(),
			ExternalIP:       data.ExternalIP(),
			VipMap:           data.VipMap(),
		}, cc)
		if err != nil {
			return err
		}
	}

	return nil
}

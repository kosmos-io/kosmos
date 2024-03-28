package cert

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type CertStore interface {
	AddCert(cert *VirtualClusterCert)
	GetCert(name string) *VirtualClusterCert
	CertList() []*VirtualClusterCert
	LoadCertFromSecret(secret *corev1.Secret) error
}

type splitToPairNameFunc func(name string) string

type VirtualClusterCertStore struct {
	certs        map[string]*VirtualClusterCert
	pairNameFunc splitToPairNameFunc
}

func NewCertStore() CertStore {
	return &VirtualClusterCertStore{
		certs:        make(map[string]*VirtualClusterCert),
		pairNameFunc: SplitToPairName,
	}
}

func SplitToPairName(name string) string {
	if strings.Contains(name, keyExtension) {
		strArr := strings.Split(name, keyExtension)
		return strArr[0]
	}

	if strings.Contains(name, certExtension) {
		strArr := strings.Split(name, certExtension)
		return strArr[0]
	}

	return name
}

func (store *VirtualClusterCertStore) AddCert(cert *VirtualClusterCert) {
	store.certs[cert.pairName] = cert
}

func (store *VirtualClusterCertStore) GetCert(name string) *VirtualClusterCert {
	for _, c := range store.certs {
		if c.pairName == name {
			return c
		}
	}
	return nil
}

func (store *VirtualClusterCertStore) CertList() []*VirtualClusterCert {
	certs := make([]*VirtualClusterCert, 0, len(store.certs))

	for _, c := range store.certs {
		certs = append(certs, c)
	}

	return certs
}

func (store *VirtualClusterCertStore) LoadCertFromSecret(secret *corev1.Secret) error {
	if len(secret.Data) == 0 {
		return fmt.Errorf("cert data is empty")
	}

	for name, data := range secret.Data {
		pairName := store.pairNameFunc(name)
		kc := store.GetCert(pairName)
		if kc == nil {
			kc = &VirtualClusterCert{
				pairName: pairName,
			}
		}

		if strings.Contains(name, certExtension) {
			kc.cert = data
		}
		if strings.Contains(name, keyExtension) {
			kc.key = data
		}

		store.AddCert(kc)
	}

	return nil
}

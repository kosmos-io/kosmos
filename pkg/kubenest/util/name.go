package util

import (
	"fmt"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
)

func GetAPIServerName(name string) string {
	return fmt.Sprintf("%s-%s", name, "apiserver")
}

func GetEtcdClientServerName(name string) string {
	return fmt.Sprintf("%s-%s", name, "etcd-client")
}

func GetKonnectivityServerName(name string) string {
	return fmt.Sprintf("%s-%s", name, "konnectivity-server")
}

func GetKonnectivityAPIServerName(name string) string {
	return fmt.Sprintf("%s-%s-konnectivity", name, "apiserver")
}

func GetEtcdServerName(name string) string {
	return fmt.Sprintf("%s-%s", name, "etcd")
}

func GetCertName(name string) string {
	return fmt.Sprintf("%s-%s", name, "cert")
}

func GetEtcdCertName(name string) string {
	return fmt.Sprintf("%s-%s", name, "etcd-cert")
}

func GetAdminConfigSecretName(name string) string {
	return fmt.Sprintf("%s-%s", name, constants.AdminConfig)
}

func GetAdminConfigClusterIPSecretName(name string) string {
	return fmt.Sprintf("%s-%s", name, "admin-config-clusterip")
}

package util

import (
	"os"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func GetImageMessage() (imageRepository string, imageVersion string) {
	imageRepository = os.Getenv(constants.DefaultImageRepositoryEnv)
	if len(imageRepository) == 0 {
		imageRepository = utils.DefaultImageRepository
	}
	imageVersion = os.Getenv(constants.DefaultImageVersionEnv)
	if len(imageVersion) == 0 {
		imageVersion = utils.DefaultImageVersion
	}
	return imageRepository, imageVersion
}

func GetCoreDNSImageTag() string {
	coreDNSImageTag := os.Getenv(constants.DefaultCoreDNSImageTagEnv)
	if coreDNSImageTag == "" {
		coreDNSImageTag = utils.DefaultCoreDNSImageTag
	}
	return coreDNSImageTag
}

func GetVirtualControllerLabel() string {
	lb := os.Getenv(constants.DefaultVirtualControllerLabelEnv)
	if len(lb) == 0 {
		return utils.LabelNodeRoleControlPlane
	}
	return lb
}

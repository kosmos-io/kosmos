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

func GetCoreDnsImageTag() string {
	coreDnsImageTag := os.Getenv(constants.DefaultCoreDnsImageTagEnv)
	if coreDnsImageTag == "" {
		coreDnsImageTag = utils.DefaultCoreDnsImageTag
	}
	return coreDnsImageTag
}

package util

import (
	"os"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func GetImageMessage() (imageRepository string, imageVersion string) {
	imageRepository = os.Getenv(constants.DefauleImageRepositoryEnv)
	if len(imageRepository) == 0 {
		imageRepository = utils.DefaultImageRepository
	}
	imageVersion = os.Getenv(constants.DefauleImageVersionEnv)
	if len(imageVersion) == 0 {
		imageVersion = utils.DefaultImageVersion
	}
	return imageRepository, imageVersion
}

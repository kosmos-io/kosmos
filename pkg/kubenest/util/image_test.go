package util

import (
	"os"
	"testing"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func TestGetImageMessage(t *testing.T) {
	// Set up environment variables for the test
	defaultRepo := "custom-repo"
	defaultVersion := "custom-version"

	os.Setenv(constants.DefaultImageRepositoryEnv, defaultRepo)
	os.Setenv(constants.DefaultImageVersionEnv, defaultVersion)

	defer func() {
		// Cleanup environment variables after test
		os.Unsetenv(constants.DefaultImageRepositoryEnv)
		os.Unsetenv(constants.DefaultImageVersionEnv)
	}()

	// Test case where env variables are set
	repo, version := GetImageMessage()
	if repo != defaultRepo {
		t.Errorf("GetImageMessage() repo = %v, want %v", repo, defaultRepo)
	}
	if version != defaultVersion {
		t.Errorf("GetImageMessage() version = %v, want %v", version, defaultVersion)
	}

	// Test case where env variables are not set
	os.Unsetenv(constants.DefaultImageRepositoryEnv)
	os.Unsetenv(constants.DefaultImageVersionEnv)

	repo, version = GetImageMessage()
	if repo != utils.DefaultImageRepository {
		t.Errorf("GetImageMessage() repo = %v, want %v", repo, utils.DefaultImageRepository)
	}
	if version != utils.DefaultImageVersion {
		t.Errorf("GetImageMessage() version = %v, want %v", version, utils.DefaultImageVersion)
	}
}

func TestGetCoreDNSImageTag(t *testing.T) {
	// Set up environment variable for the test
	defaultCoreDNSImageTag := "custom-coredns-tag"
	os.Setenv(constants.DefaultCoreDNSImageTagEnv, defaultCoreDNSImageTag)

	defer func() {
		// Cleanup environment variable after test
		os.Unsetenv(constants.DefaultCoreDNSImageTagEnv)
	}()

	// Test case where env variable is set
	coreDNSImageTag := GetCoreDNSImageTag()
	if coreDNSImageTag != defaultCoreDNSImageTag {
		t.Errorf("GetCoreDNSImageTag() = %v, want %v", coreDNSImageTag, defaultCoreDNSImageTag)
	}

	// Test case where env variable is not set
	os.Unsetenv(constants.DefaultCoreDNSImageTagEnv)
	coreDNSImageTag = GetCoreDNSImageTag()
	if coreDNSImageTag != utils.DefaultCoreDNSImageTag {
		t.Errorf("GetCoreDNSImageTag() = %v, want %v", coreDNSImageTag, utils.DefaultCoreDNSImageTag)
	}
}

func TestGetVirtualControllerLabel(t *testing.T) {
	// Set up environment variable for the test
	defaultLabel := "custom-label"
	os.Setenv(constants.DefaultVirtualControllerLabelEnv, defaultLabel)

	defer func() {
		// Cleanup environment variable after test
		os.Unsetenv(constants.DefaultVirtualControllerLabelEnv)
	}()

	// Test case where env variable is set
	label := GetVirtualControllerLabel()
	if label != defaultLabel {
		t.Errorf("GetVirtualControllerLabel() = %v, want %v", label, defaultLabel)
	}

	// Test case where env variable is not set
	os.Unsetenv(constants.DefaultVirtualControllerLabelEnv)
	label = GetVirtualControllerLabel()
	if label != utils.LabelNodeRoleControlPlane {
		t.Errorf("GetVirtualControllerLabel() = %v, want %v", label, utils.LabelNodeRoleControlPlane)
	}
}

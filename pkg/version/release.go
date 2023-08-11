package version

import (
	"fmt"

	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/klog/v2"
)

// ReleaseVersion represents a released version.
type ReleaseVersion struct {
	*utilversion.Version
}

// ParseGitVersion parses a git version string, such as:
// - v1.1.0-73-g7e6d4f69
// - v1.1.0
func ParseGitVersion(gitVersion string) (*ReleaseVersion, error) {
	v, err := utilversion.ParseSemantic(gitVersion)
	if err != nil {
		return nil, err
	}

	return &ReleaseVersion{
		Version: v,
	}, nil
}

// FirstMinorRelease returns the minor release but the patch releases always be 0(vx.y.0). e.g:
// - v1.2.1-12-g2eb92858 --> v1.2.0
// - v1.2.3-12-g2e860210 --> v1.2.0
func (r *ReleaseVersion) FirstMinorRelease() string {
	if r.Version == nil {
		return "<nil>"
	}

	return fmt.Sprintf("v%d.%d.0", r.Version.Major(), r.Version.Minor())
}

// PatchRelease returns the stable version with format "vx.y.z".
func (r *ReleaseVersion) PatchRelease() string {
	if r.Version == nil {
		return "<nil>"
	}

	versionStr := fmt.Sprintf("%d.%d.%d", r.Version.Major(), r.Version.Minor(), r.Version.Patch())
	if len(r.Version.PreRelease()) > 0 {
		versionStr = fmt.Sprintf("%s-%s", versionStr, r.Version.PreRelease())
	}
	if len(r.Version.BuildMetadata()) > 0 {
		versionStr = fmt.Sprintf("%s-%s", versionStr, r.Version.BuildMetadata())
	}
	return versionStr
}

func GetReleaseVersion() *ReleaseVersion {
	releaseVer, err := ParseGitVersion(Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", Get().String())
		releaseVer, err = ParseGitVersion("0.1.0")
		if err != nil {
			klog.Warningf("ParseGitVersion err: %v", err)
		}
	}
	return releaseVer
}

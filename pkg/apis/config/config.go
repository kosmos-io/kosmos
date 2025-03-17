package config

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const Version = "v1beta1"

type Config struct {
	// Version is the config version
	Version string `yaml:"version,omitempty" json:"version,omitempty"`

	// Mappings defines a way to map a resource to another resource
	Mappings []Mapping `yaml:"mappings,omitempty" json:"mappings,omitempty"`
}

type Mapping struct {
	// FromHost syncs a resource from the host to the leaf cluster
	FromHostCluster *FromHostCluster `yaml:"fromHostCluster,omitempty" json:"fromHostCluster,omitempty"`
}

type TypeInformation struct {
	// APIVersion of the object to sync
	APIVersion string `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`

	// Kind of the object to sync
	Kind string `yaml:"kind,omitempty" json:"kind,omitempty"`
}

type SyncBase struct {
	TypeInformation `yaml:",inline" json:",inline"`

	// ID is the id of the controller. This is optional and only necessary if you have multiple fromHostCluster
	// controllers that target the same group version kind.
	ID string `yaml:"id,omitempty" json:"id,omitempty"`
}

type FromHostCluster struct {
	SyncBase `yaml:",inline" json:",inline"`
}

func ParseConfig(rawConfig string) (*Config, error) {
	configuration := &Config{}
	err := yaml.UnmarshalStrict([]byte(rawConfig), configuration)
	if err != nil {
		return nil, errors.Wrap(err, "parse config")
	}

	err = validate(configuration)
	if err != nil {
		return nil, err
	}

	return configuration, nil
}

func validate(config *Config) error {
	if config.Version != Version {
		return fmt.Errorf("unsupported configuration version. %s", config.Version)
	}
	for idx, mapping := range config.Mappings {
		if mapping.FromHostCluster == nil {
			return fmt.Errorf("mapping must have a fromHostCluster")
		}
		if mapping.FromHostCluster.Kind == "" {
			return fmt.Errorf("mapping %d must have a namespace", idx)
		}
		if mapping.FromHostCluster.APIVersion == "" {
			return fmt.Errorf("mapping %d must have a apiVersion", idx)
		}
	}
	return nil
}

//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by conversion-gen. DO NOT EDIT.

package v1beta1

import (
	config "github.com/kosmos.io/kosmos/pkg/apis/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*LeafNodeDistributionArgs)(nil), (*config.LeafNodeDistributionArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_LeafNodeDistributionArgs_To_config_LeafNodeDistributionArgs(a.(*LeafNodeDistributionArgs), b.(*config.LeafNodeDistributionArgs), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.LeafNodeDistributionArgs)(nil), (*LeafNodeDistributionArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_LeafNodeDistributionArgs_To_v1beta1_LeafNodeDistributionArgs(a.(*config.LeafNodeDistributionArgs), b.(*LeafNodeDistributionArgs), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*LeafNodeVolumeBindingArgs)(nil), (*config.LeafNodeVolumeBindingArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_LeafNodeVolumeBindingArgs_To_config_LeafNodeVolumeBindingArgs(a.(*LeafNodeVolumeBindingArgs), b.(*config.LeafNodeVolumeBindingArgs), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.LeafNodeVolumeBindingArgs)(nil), (*LeafNodeVolumeBindingArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_LeafNodeVolumeBindingArgs_To_v1beta1_LeafNodeVolumeBindingArgs(a.(*config.LeafNodeVolumeBindingArgs), b.(*LeafNodeVolumeBindingArgs), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*LeafNodeWorkloadArgs)(nil), (*config.LeafNodeWorkloadArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_LeafNodeWorkloadArgs_To_config_LeafNodeWorkloadArgs(a.(*LeafNodeWorkloadArgs), b.(*config.LeafNodeWorkloadArgs), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.LeafNodeWorkloadArgs)(nil), (*LeafNodeWorkloadArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_LeafNodeWorkloadArgs_To_v1beta1_LeafNodeWorkloadArgs(a.(*config.LeafNodeWorkloadArgs), b.(*LeafNodeWorkloadArgs), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta1_LeafNodeDistributionArgs_To_config_LeafNodeDistributionArgs(in *LeafNodeDistributionArgs, out *config.LeafNodeDistributionArgs, s conversion.Scope) error {
	out.KubeConfigPath = in.KubeConfigPath
	return nil
}

// Convert_v1beta1_LeafNodeDistributionArgs_To_config_LeafNodeDistributionArgs is an autogenerated conversion function.
func Convert_v1beta1_LeafNodeDistributionArgs_To_config_LeafNodeDistributionArgs(in *LeafNodeDistributionArgs, out *config.LeafNodeDistributionArgs, s conversion.Scope) error {
	return autoConvert_v1beta1_LeafNodeDistributionArgs_To_config_LeafNodeDistributionArgs(in, out, s)
}

func autoConvert_config_LeafNodeDistributionArgs_To_v1beta1_LeafNodeDistributionArgs(in *config.LeafNodeDistributionArgs, out *LeafNodeDistributionArgs, s conversion.Scope) error {
	out.KubeConfigPath = in.KubeConfigPath
	return nil
}

// Convert_config_LeafNodeDistributionArgs_To_v1beta1_LeafNodeDistributionArgs is an autogenerated conversion function.
func Convert_config_LeafNodeDistributionArgs_To_v1beta1_LeafNodeDistributionArgs(in *config.LeafNodeDistributionArgs, out *LeafNodeDistributionArgs, s conversion.Scope) error {
	return autoConvert_config_LeafNodeDistributionArgs_To_v1beta1_LeafNodeDistributionArgs(in, out, s)
}

func autoConvert_v1beta1_LeafNodeVolumeBindingArgs_To_config_LeafNodeVolumeBindingArgs(in *LeafNodeVolumeBindingArgs, out *config.LeafNodeVolumeBindingArgs, s conversion.Scope) error {
	out.BindTimeoutSeconds = in.BindTimeoutSeconds
	return nil
}

// Convert_v1beta1_LeafNodeVolumeBindingArgs_To_config_LeafNodeVolumeBindingArgs is an autogenerated conversion function.
func Convert_v1beta1_LeafNodeVolumeBindingArgs_To_config_LeafNodeVolumeBindingArgs(in *LeafNodeVolumeBindingArgs, out *config.LeafNodeVolumeBindingArgs, s conversion.Scope) error {
	return autoConvert_v1beta1_LeafNodeVolumeBindingArgs_To_config_LeafNodeVolumeBindingArgs(in, out, s)
}

func autoConvert_config_LeafNodeVolumeBindingArgs_To_v1beta1_LeafNodeVolumeBindingArgs(in *config.LeafNodeVolumeBindingArgs, out *LeafNodeVolumeBindingArgs, s conversion.Scope) error {
	out.BindTimeoutSeconds = in.BindTimeoutSeconds
	return nil
}

// Convert_config_LeafNodeVolumeBindingArgs_To_v1beta1_LeafNodeVolumeBindingArgs is an autogenerated conversion function.
func Convert_config_LeafNodeVolumeBindingArgs_To_v1beta1_LeafNodeVolumeBindingArgs(in *config.LeafNodeVolumeBindingArgs, out *LeafNodeVolumeBindingArgs, s conversion.Scope) error {
	return autoConvert_config_LeafNodeVolumeBindingArgs_To_v1beta1_LeafNodeVolumeBindingArgs(in, out, s)
}

func autoConvert_v1beta1_LeafNodeWorkloadArgs_To_config_LeafNodeWorkloadArgs(in *LeafNodeWorkloadArgs, out *config.LeafNodeWorkloadArgs, s conversion.Scope) error {
	if err := v1.Convert_Pointer_string_To_string(&in.KubeConfigPath, &out.KubeConfigPath, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1beta1_LeafNodeWorkloadArgs_To_config_LeafNodeWorkloadArgs is an autogenerated conversion function.
func Convert_v1beta1_LeafNodeWorkloadArgs_To_config_LeafNodeWorkloadArgs(in *LeafNodeWorkloadArgs, out *config.LeafNodeWorkloadArgs, s conversion.Scope) error {
	return autoConvert_v1beta1_LeafNodeWorkloadArgs_To_config_LeafNodeWorkloadArgs(in, out, s)
}

func autoConvert_config_LeafNodeWorkloadArgs_To_v1beta1_LeafNodeWorkloadArgs(in *config.LeafNodeWorkloadArgs, out *LeafNodeWorkloadArgs, s conversion.Scope) error {
	if err := v1.Convert_string_To_Pointer_string(&in.KubeConfigPath, &out.KubeConfigPath, s); err != nil {
		return err
	}
	return nil
}

// Convert_config_LeafNodeWorkloadArgs_To_v1beta1_LeafNodeWorkloadArgs is an autogenerated conversion function.
func Convert_config_LeafNodeWorkloadArgs_To_v1beta1_LeafNodeWorkloadArgs(in *config.LeafNodeWorkloadArgs, out *LeafNodeWorkloadArgs, s conversion.Scope) error {
	return autoConvert_config_LeafNodeWorkloadArgs_To_v1beta1_LeafNodeWorkloadArgs(in, out, s)
}

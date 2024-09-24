// nolint:dupl
package flags

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestAddFlags(t *testing.T) {
	t.Run("AddFlags with default values", func(t *testing.T) {
		o := &BackoffOptions{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		o.AddFlags(fs)

		// Check default values
		if o.Duration != 5*time.Millisecond {
			t.Errorf("Expected default Duration to be 5ms, got %v", o.Duration)
		}
		if o.Factor != 1.0 {
			t.Errorf("Expected default Factor to be 1.0, got %v", o.Factor)
		}
		if o.Jitter != 0.1 {
			t.Errorf("Expected default Jitter to be 0.1, got %v", o.Jitter)
		}
		if o.Steps != 5 {
			t.Errorf("Expected default Steps to be 5, got %v", o.Steps)
		}
	})

	t.Run("AddFlags with custom values", func(t *testing.T) {
		o := &BackoffOptions{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		o.AddFlags(fs)

		// Check setting values
		err := fs.Parse([]string{"--retry-duration=1s", "--retry-factor=2.0", "--retry-jitter=0.2", "--retry-steps=10"})
		if err != nil {
			t.Errorf("Expected no error, but got %v", err)
		}

		if o.Duration != 1*time.Second {
			t.Errorf("Expected Duration to be 1s, got %v", o.Duration)
		}
		if o.Factor != 2.0 {
			t.Errorf("Expected Factor to be 2.0, got %v", o.Factor)
		}
		if o.Jitter != 0.2 {
			t.Errorf("Expected Jitter to be 0.2, got %v", o.Jitter)
		}
		if o.Steps != 10 {
			t.Errorf("Expected Steps to be 10, got %v", o.Steps)
		}
	})

	t.Run("AddFlags with zero values", func(t *testing.T) {
		o := &BackoffOptions{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		o.AddFlags(fs)

		// Check setting values
		err := fs.Parse([]string{"--retry-duration=0", "--retry-factor=0.0", "--retry-jitter=0.0", "--retry-steps=0"})
		if err != nil {
			t.Errorf("Expected no error, but got %v", err)
		}

		if o.Duration != 0 {
			t.Errorf("Expected Duration to be 0, got %v", o.Duration)
		}
		if o.Factor != 0.0 {
			t.Errorf("Expected Factor to be 0.0, got %v", o.Factor)
		}
		if o.Jitter != 0.0 {
			t.Errorf("Expected Jitter to be 0.0, got %v", o.Jitter)
		}
		if o.Steps != 0 {
			t.Errorf("Expected Steps to be 0, got %v", o.Steps)
		}
	})
}

func TestDefaultUpdateRetryBackoff(t *testing.T) {
	t.Run("Test with zero values", func(t *testing.T) {
		opts := BackoffOptions{}
		backoff := DefaultUpdateRetryBackoff(opts)

		if backoff.Duration != 5*time.Millisecond {
			t.Errorf("Expected duration to be 5ms, got %v", backoff.Duration)
		}
		if backoff.Factor != 1.0 {
			t.Errorf("Expected factor to be 1.0, got %v", backoff.Factor)
		}
		if backoff.Jitter != 0.1 {
			t.Errorf("Expected jitter to be 0.1, got %v", backoff.Jitter)
		}
		if backoff.Steps != 5 {
			t.Errorf("Expected steps to be 5, got %v", backoff.Steps)
		}
	})

	t.Run("Test with custom values", func(t *testing.T) {
		opts := BackoffOptions{
			Duration: 10 * time.Millisecond,
			Factor:   2.0,
			Jitter:   0.2,
			Steps:    10,
		}
		backoff := DefaultUpdateRetryBackoff(opts)

		if backoff.Duration != 10*time.Millisecond {
			t.Errorf("Expected duration to be 10ms, got %v", backoff.Duration)
		}
		if backoff.Factor != 2.0 {
			t.Errorf("Expected factor to be 2.0, got %v", backoff.Factor)
		}
		if backoff.Jitter != 0.2 {
			t.Errorf("Expected jitter to be 0.2, got %v", backoff.Jitter)
		}
		if backoff.Steps != 10 {
			t.Errorf("Expected steps to be 10, got %v", backoff.Steps)
		}
	})

	t.Run("Test with negative values", func(t *testing.T) {
		opts := BackoffOptions{
			Duration: -1 * time.Millisecond,
			Factor:   -2.0,
			Jitter:   -0.2,
			Steps:    -10,
		}
		backoff := DefaultUpdateRetryBackoff(opts)

		if backoff.Duration != 5*time.Millisecond {
			t.Errorf("Expected duration to be 5ms, got %v", backoff.Duration)
		}
		if backoff.Factor != 1.0 {
			t.Errorf("Expected factor to be 1.0, got %v", backoff.Factor)
		}
		if backoff.Jitter != 0.1 {
			t.Errorf("Expected jitter to be 0.1, got %v", backoff.Jitter)
		}
		if backoff.Steps != 5 {
			t.Errorf("Expected steps to be 5, got %v", backoff.Steps)
		}
	})

	t.Run("Test with part values", func(t *testing.T) {
		opts := BackoffOptions{
			Jitter: 0.2,
			Steps:  10,
		}
		backoff := DefaultUpdateRetryBackoff(opts)

		if backoff.Duration != 5*time.Millisecond {
			t.Errorf("Expected duration to be 5ms, got %v", backoff.Duration)
		}
		if backoff.Factor != 1.0 {
			t.Errorf("Expected factor to be 1.0, got %v", backoff.Factor)
		}
		if backoff.Jitter != 0.2 {
			t.Errorf("Expected jitter to be 0.2, got %v", backoff.Jitter)
		}
		if backoff.Steps != 10 {
			t.Errorf("Expected steps to be 10, got %v", backoff.Steps)
		}
	})
}

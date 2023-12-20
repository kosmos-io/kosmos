package flags

import (
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/wait"
)

// BackoffOptions are options for retry flag.
type BackoffOptions struct {
	// The initial duration.
	Duration time.Duration
	// Duration is multiplied by factor each iteration, if factor is not zero
	// and the limits imposed by Steps and Cap have not been reached.
	// Should not be negative.
	// The jitter does not contribute to the updates to the duration parameter.
	Factor float64
	// The sleep at each iteration is the duration plus an additional
	// amount chosen uniformly at random from the interval between
	// zero and `jitter*duration`.
	Jitter float64
	// The remaining number of iterations in which the duration
	// parameter may change (but progress can be stopped earlier by
	// hitting the cap). If not positive, the duration is not
	// changed. Used for exponential backoff in combination with
	// Factor and Cap.
	Steps int
}

// AddFlags adds flags to the specified FlagSet.
func (o *BackoffOptions) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.Duration, "retry-duration", 2*time.Second, "the retry duration.")
	fs.Float64Var(&o.Factor, "retry-factor", 1.0, "Duration is multiplied by factor each iteration.")
	fs.Float64Var(&o.Jitter, "retry-jitter", 0.2, "The sleep at each iteration is the duration plus an additional amount.")
	fs.IntVar(&o.Steps, "retry-steps", 5, "The retry steps.")
}

// DefaultUpdateRetryBackoff provide a default retry function for update resources
func DefaultUpdateRetryBackoff(opts BackoffOptions) wait.Backoff {
	// set defaults
	if opts.Duration <= 0 {
		opts.Duration = 2 * time.Second
	}
	if opts.Factor <= 0 {
		opts.Factor = 1.0
	}
	if opts.Jitter <= 0 {
		opts.Jitter = 0.1
	}
	if opts.Steps <= 0 {
		opts.Steps = 5
	}
	return wait.Backoff{
		Steps:    opts.Steps,
		Duration: opts.Duration,
		Factor:   opts.Factor,
		Jitter:   opts.Jitter,
	}
}

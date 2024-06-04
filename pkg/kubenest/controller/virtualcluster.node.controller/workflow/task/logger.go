package task

import (
	"fmt"

	"k8s.io/klog/v2"
)

type PrefixedLogger struct {
	level  klog.Verbose
	prefix string
}

func NewPrefixedLogger(level klog.Verbose, prefix string) *PrefixedLogger {
	return &PrefixedLogger{level: level, prefix: prefix}
}

func (p *PrefixedLogger) Info(args ...interface{}) {
	if p.level.Enabled() {
		klog.InfoDepth(1, append([]interface{}{p.prefix}, args...)...)
	}
}

func (p *PrefixedLogger) Infof(format string, args ...interface{}) {
	if p.level.Enabled() {
		klog.InfoDepth(1, fmt.Sprintf(p.prefix+format, args...))
	}
}

func (p *PrefixedLogger) Error(args ...interface{}) {
	klog.ErrorDepth(1, append([]interface{}{p.prefix}, args...)...)
}

func (p *PrefixedLogger) Errorf(format string, args ...interface{}) {
	klog.ErrorDepth(1, fmt.Sprintf(p.prefix+format, args...))
}

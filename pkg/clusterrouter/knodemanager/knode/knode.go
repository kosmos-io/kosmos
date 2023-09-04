package knode

import (
	"context"

	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/config"
)

type Knode struct {
}

func NewKnode(_ context.Context, _ *string, _ *config.Opts) (*Knode, error) {
	return &Knode{}, nil
}

func (kn *Knode) Run(_ context.Context, _ *config.Opts) {
}

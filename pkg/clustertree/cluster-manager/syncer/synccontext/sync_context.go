package synccontext

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SyncContext struct {
	context.Context

	RootClient client.Client
	LeafClient client.Client

	CurrentCrdName string

	RootManager ctrl.Manager

	LeafManager ctrl.Manager
}

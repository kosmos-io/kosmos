package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PromotePolicySpec defines the desired state of promotePolicy
type PromotePolicySpec struct {
	// Cluster is a cluster that needs to be migrated
	ClusterName string `json:"clusterName,omitempty"`

	// IncludedNamespaces is a slice of namespace names to include objects
	// from. If empty, all namespaces are included.
	// +optional
	// +nullable
	IncludedNamespaces []string `json:"includedNamespaces,omitempty"`

	// ExcludedNamespaces contains a list of namespaces that are not
	// included in the backup.
	// +optional
	// +nullable
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`

	// IncludedNamespaceScopedResources is a slice of namespace-scoped
	// resource type names to include in the backup.
	// The default value is "*".
	// +optional
	// +nullable
	IncludedNamespaceScopedResources []string `json:"includedNamespaceScopedResources,omitempty"`

	// ExcludedNamespaceScopedResources is a slice of namespace-scoped
	// resource type names to exclude from the backup.
	// If set to "*", all namespace-scoped resource types are excluded.
	// The default value is empty.
	// +optional
	// +nullable
	ExcludedNamespaceScopedResources []string `json:"excludedNamespaceScopedResources,omitempty"`

	// Rollback set true, then rollback from the backup
	// +optional
	// +nullable
	Rollback string `json:"rollback,omitempty"`
}

// PromotePolicyPhase is a string representation of the lifecycle phase
type PromotePolicyPhase string

const (
	// PromotePolicyPhasePrecheck means in precheck progess
	PromotePolicyPhasePrecheck PromotePolicyPhase = "Prechecking"

	// PromotePolicyPhaseFailedPrecheck means precheck has failed
	PromotePolicyPhaseFailedPrecheck PromotePolicyPhase = "FailedPrecheck"

	// PromotePolicyPhaseBackup means in backup progess
	PromotePolicyPhaseBackup PromotePolicyPhase = "Backuping"

	// PromotePolicyPhaseFailedBackup means backup has failed
	PromotePolicyPhaseFailedBackup PromotePolicyPhase = "FailedBackup"

	// PromotePolicyPhaseDetach means in detach progess
	PromotePolicyPhaseDetach PromotePolicyPhase = "Detaching"

	// PromotePolicyPhaseFailedDetach means detach has failed
	PromotePolicyPhaseFailedDetach PromotePolicyPhase = "FailedDetach"

	// PromotePolicyPhaseRestore means in restore progess
	PromotePolicyPhaseRestore PromotePolicyPhase = "Restoring"

	// PromotePolicyPhaseFailedRestore means restore has failed
	PromotePolicyPhaseFailedRestore PromotePolicyPhase = "FailedRestore"

	// PromotePolicyPhaseFailedRollback means rollback has failed
	PromotePolicyPhaseFailedRollback PromotePolicyPhase = "FailedRollback"

	// PromotePolicyPhaseRolledback means rollback has successed
	PromotePolicyPhaseRolledback PromotePolicyPhase = "RolledBack"

	// PromotePolicyPhaseCompleted means the sync has run successfully
	PromotePolicyPhaseCompleted PromotePolicyPhase = "Completed"
)

// BackupProgress stores information about the progress of a Backup's execution.
type PromotePolicyProgress struct {
	// TotalItems is the total number of items to be backed up. This number may change
	// throughout the execution of the backup due to plugins that return additional related
	// items to back up, the velero.io/exclude-from-backup label, and various other
	// filters that happen as items are processed.
	// +optional
	TotalItems int `json:"totalItems,omitempty"`

	// ItemsBackedUp is the number of items that have actually been written to the
	// backup tarball so far.
	// +optional
	ItemsBackedUp int `json:"itemsBackedUp,omitempty"`
}

// PromotePolicyStatus defines the observed state of promotePolicy
type PromotePolicyStatus struct {
	// Phase is the current state of the Backup.
	// +optional
	Phase PromotePolicyPhase `json:"phase,omitempty"`

	// PrecheckErrors is a slice of all precheck errors (if
	// applicable).
	// +optional
	// +nullable
	PrecheckErrors []string `json:"precheckErrors,omitempty"`

	// StartTimestamp records the time a sync was started.
	// The server's time is used for StartTimestamps
	// +optional
	// +nullable
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp records the time a sync was completed.
	// Completion time is recorded even on failed sync.
	// The server's time is used for CompletionTimestamps
	// +optional
	// +nullable
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// FailureReason is an error that caused the entire sync to fail.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// Progress contains information about the sync's execution progress. Note
	// that this information is best-effort only -- if fails to update it for any reason, it may be inaccurate/stale.
	// +optional
	// +nullable
	Progress *PromotePolicyProgress `json:"progress,omitempty"`

	BackedupFile string `json:"backedupFile,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:object:generate=true
// +kubebuilder:storageversion
// +kubebuilder:rbac:groups=velero.io,resources=backups,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=velero.io,resources=backups/status,verbs=get;update;patch

// PromotePolicy is custom resource that represents the capture of sync leaf cluster
type PromotePolicy struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PromotePolicySpec `json:"spec,omitempty"`

	Status PromotePolicyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupList is a list of promotePolicys.
type PromotePolicyList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PromotePolicy `json:"items"`
}

package constants

const (
	BackupDir = "/data/backup/"

	// ResourcesDir is a top-level directory expected in backups which contains sub-directories
	// for each resource type in the backup.
	ResourcesDir = "resources"

	// MetadataDir is a top-level directory expected in backups which contains
	// files that store metadata about the backup, such as the backup version.
	MetadataDir = "metadata"

	// ClusterScopedDir is the name of the directory containing cluster-scoped
	ClusterScopedDir = "cluster"

	// NamespaceScopedDir is the name of the directory containing namespace-scoped
	NamespaceScopedDir = "namespaces"

	BackupFormatVersion = "1.1.0"

	// PreferredVersionDir is the suffix name of the directory containing the preferred version of the API group
	PreferredVersionDir = "-preferredversion"

	ItemRestoreResultCreated = "created"
	ItemRestoreResultUpdated = "updated"
	ItemRestoreResultFailed  = "failed"
	ItemRestoreResultSkipped = "skipped"
)

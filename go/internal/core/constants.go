package core

import "time"

const (
	ProjectName               = "codex"
	PublicSyncRepositoryName  = "codex-build-public"
	PublicSyncLogLimit        = 1 << 20
	SchemaVersion             = 3
	DefaultNextestConcurrency = 16
)

func StageDeadline() time.Duration {
	return 8 * time.Hour
}

func CleanupDeadline() time.Duration {
	return 10 * time.Minute
}

func GracePeriod() time.Duration {
	return 10 * time.Second
}

func ResourceLimitArgs() []string {
	return []string{
		"--cpu=28800",
		"--as=68719476736",
		"--fsize=214748364800",
		"--nofile=65536",
	}
}

package data

import "time"

type Config struct {
	DefaultLocalRetentionTTL time.Duration
	DefaultRelayRetentionTTL time.Duration
	MaxRelayRetentionBytes   int64
	MaxReplicaRetentionBytes int64
	MaxLocalStorageBytes     int64
	DefaultDesiredReplicas   int
	DefaultMinimumReplicas   int
}

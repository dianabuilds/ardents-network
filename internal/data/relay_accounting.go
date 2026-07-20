package data

import retentionpkg "ardents/internal/data/retention"

func (s *Service) relayBytesWithoutLocked(excludeID string) int64 {
	return retentionpkg.RelayBytes(&s.blobs, excludeID, s.localPayloadInfoLocked)
}

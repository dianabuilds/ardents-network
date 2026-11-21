//go:build linux

package release

import (
	"encoding/json"
	"errors"
	"fmt"
)

// StaticMetadataFiles adapts four fixed public files to the existing offline
// fetcher's URLs, including consistent-snapshot names. It supplies candidate
// locations only; normal Release signatures, versions, expiry and bounds still
// authenticate every byte. No root rotation file is invented or downloaded.
func StaticMetadataFiles(root, timestamp, snapshot, targets []byte) (map[string][]byte, error) {
	for _, raw := range [][]byte{root, timestamp, snapshot, targets} {
		if len(raw) == 0 || len(raw) > int(maximumMetadataFileBytes) || validateJSONObjects(raw, true) != nil {
			return nil, errors.New("release: invalid static metadata envelope")
		}
	}
	var r struct {
		Signed struct {
			Consistent bool `json:"consistent_snapshot"`
		} `json:"signed"`
	}
	var ts struct {
		Signed struct {
			Meta map[string]struct {
				Version int64 `json:"version"`
			} `json:"meta"`
		} `json:"signed"`
	}
	var sn struct {
		Signed struct {
			Meta map[string]struct {
				Version int64 `json:"version"`
			} `json:"meta"`
		} `json:"signed"`
	}
	if json.Unmarshal(root, &r) != nil || json.Unmarshal(timestamp, &ts) != nil || json.Unmarshal(snapshot, &sn) != nil {
		return nil, errors.New("release: invalid static metadata references")
	}
	const base = "https://release.invalid/metadata/"
	files := map[string][]byte{base + "timestamp.json": append([]byte(nil), timestamp...)}
	if r.Signed.Consistent {
		sv, tv := ts.Signed.Meta["snapshot.json"].Version, sn.Signed.Meta["targets.json"].Version
		if sv <= 0 || tv <= 0 {
			return nil, errors.New("release: invalid static metadata versions")
		}
		files[fmt.Sprintf("%s%d.snapshot.json", base, sv)] = append([]byte(nil), snapshot...)
		files[fmt.Sprintf("%s%d.targets.json", base, tv)] = append([]byte(nil), targets...)
	} else {
		files[base+"snapshot.json"] = append([]byte(nil), snapshot...)
		files[base+"targets.json"] = append([]byte(nil), targets...)
	}
	return files, nil
}

package recovery

import (
	"bytes"
	"encoding/json"
	"io"
)

type dockerCleanupProjection struct {
	Project                       string
	Containers, Networks, Volumes uint32
}

func validDockerCleanupProjection(value cleanup, scope hostScopeEvidence) bool {
	var projection dockerCleanupProjection
	decoder := json.NewDecoder(bytes.NewReader(value.AdapterProjection))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&projection) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	canonical, err := json.Marshal(projection)
	return err == nil && bytes.Equal(canonical, value.AdapterProjection) &&
		projection.Project == scope.AdapterProjection && projection.Containers == 0 &&
		projection.Networks == 0 && projection.Volumes == 0
}

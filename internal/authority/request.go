package authority

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func requestPayloadHash(request CreateRequest) string {
	canonical := fmt.Sprintf("v=%d\nclass=%s", request.Version, request.RealmClass)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func validateCreateRequest(request CreateRequest) error {
	if request.Version != ContractVersion {
		return ErrUnsupportedVersion
	}
	if len(request.RequestID) == 0 || len(request.RequestID) > MaxRequestIDBytes ||
		strings.TrimSpace(request.RequestID) != request.RequestID {
		return ErrInvalidArgument
	}
	return nil
}

func validateCreatePayload(request CreateRequest) error {
	if request.RealmClass != RealmClassProduction {
		return ErrInvalidArgument
	}
	return nil
}

func validateInspectRequest(request InspectRequest) error {
	if request.Version != ContractVersion {
		return ErrUnsupportedVersion
	}
	if !ValidRealmID(request.RealmID) {
		return ErrInvalidArgument
	}
	return nil
}

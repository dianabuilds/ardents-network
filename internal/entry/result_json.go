package entry

import (
	"encoding/hex"
	"encoding/json"
)

// MarshalJSON keeps the opaque Invite identifier bounded and textual.
func (result Result) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Class      Class  `json:"class"`
		InviteID   string `json:"invite_id"`
		Slot       uint8  `json:"slot"`
		Generation uint8  `json:"generation"`
	}{result.Class, hex.EncodeToString(result.InviteID[:]), result.Slot, result.Generation})
}

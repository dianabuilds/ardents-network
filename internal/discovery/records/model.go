package records

import "time"

const LocalRecordTTL = 24 * time.Hour

type Record struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Subject   string    `json:"subject"`
	Node      string    `json:"node"`
	Device    string    `json:"device"`
	Owner     string    `json:"owner,omitempty"`
	Service   string    `json:"service,omitempty"`
	Mode      string    `json:"mode,omitempty"`
	PublicKey string    `json:"public_key"`
	Endpoints []string  `json:"endpoints"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Signature string    `json:"signature"`
}

type Entry struct {
	Record Record    `json:"record"`
	Source string    `json:"source"`
	SeenAt time.Time `json:"seen_at"`
}

package records

import (
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

const (
	Version        uint32 = 1
	KindNode              = "node"
	KindService           = "service"
	LocalRecordTTL        = 24 * time.Hour
)

type ServiceID string
type WorkloadID string

type NodeFacts struct {
	Principal identityprincipal.ID `json:"principal"`
	PublicKey string               `json:"public_key"`
	Endpoints []string             `json:"endpoints"`
}

type ServiceFacts struct {
	ID            ServiceID            `json:"service_id"`
	Type          string               `json:"service_type"`
	NodePrincipal identityprincipal.ID `json:"node_principal"`
	Workload      WorkloadID           `json:"workload_id"`
	Mode          string               `json:"mode"`
	PublicKey     string               `json:"public_key"`
	Endpoints     []string             `json:"endpoints"`
}

// Record is the signed versioned discovery envelope. Exactly one facts body is
// present; source and observation time belong to Entry and are never signed.
type Record struct {
	Version   uint32        `json:"version"`
	Node      *NodeFacts    `json:"node,omitempty"`
	Service   *ServiceFacts `json:"service,omitempty"`
	IssuedAt  time.Time     `json:"issued_at"`
	ExpiresAt time.Time     `json:"expires_at"`
	Signature string        `json:"signature"`
}

func (r Record) Kind() string {
	if r.Node != nil && r.Service == nil {
		return KindNode
	}
	if r.Service != nil && r.Node == nil {
		return KindService
	}
	return ""
}

func (r Record) RecordID() string {
	if r.Node != nil {
		return r.Node.Principal.String() + ":node"
	}
	if r.Service != nil {
		return string(r.Service.ID)
	}
	return ""
}

func (r Record) Subject() string {
	if r.Node != nil {
		return r.Node.Principal.String()
	}
	if r.Service != nil {
		return string(r.Service.ID)
	}
	return ""
}

func (r Record) NodeID() string {
	if r.Node != nil {
		return r.Node.Principal.String()
	}
	if r.Service != nil {
		return r.Service.NodePrincipal.String()
	}
	return ""
}

func (r Record) PublicKeyText() string {
	if r.Node != nil {
		return r.Node.PublicKey
	}
	if r.Service != nil {
		return r.Service.PublicKey
	}
	return ""
}

func (r Record) EndpointList() []string {
	if r.Node != nil {
		return r.Node.Endpoints
	}
	if r.Service != nil {
		return r.Service.Endpoints
	}
	return nil
}

func (r Record) ServiceType() string {
	if r.Service == nil {
		return ""
	}
	return r.Service.Type
}

func (r Record) WorkloadID() string {
	if r.Service == nil {
		return ""
	}
	return string(r.Service.Workload)
}

func (r Record) ServiceMode() string {
	if r.Service == nil {
		return ""
	}
	return r.Service.Mode
}

func (r Record) Clone() Record {
	out := r
	if r.Node != nil {
		facts := *r.Node
		facts.Endpoints = append([]string(nil), r.Node.Endpoints...)
		out.Node = &facts
	}
	if r.Service != nil {
		facts := *r.Service
		facts.Endpoints = append([]string(nil), r.Service.Endpoints...)
		out.Service = &facts
	}
	return out
}

type Entry struct {
	Record Record    `json:"record"`
	Source string    `json:"source"`
	SeenAt time.Time `json:"seen_at"`
}

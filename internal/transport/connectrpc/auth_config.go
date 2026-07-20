package connectrpc

import (
	"fmt"
	"strings"
	"time"
)

type AuthConfig struct {
	Token           string
	SubjectID       string
	Capabilities    []string
	ExpiresAt       time.Time
	TargetNode      string
	TargetPrincipal string
	Now             func() time.Time
}

func (c AuthConfig) validate() error {
	return c.Validate()
}

func (c AuthConfig) Validate() error {
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("connect api token is required")
	}
	if len(c.capabilities()) == 0 {
		return fmt.Errorf("connect api capabilities are required")
	}
	for _, capability := range c.capabilities() {
		if !knownOperatorCapability(capability) {
			return fmt.Errorf("unknown connect api capability %q", capability)
		}
	}
	return nil
}

func (c AuthConfig) subjectID() string {
	if strings.TrimSpace(c.SubjectID) != "" {
		return c.SubjectID
	}
	return "local-api"
}

func (c AuthConfig) capabilities() []string {
	return append([]string(nil), c.Capabilities...)
}

func (c AuthConfig) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func knownOperatorCapability(capability string) bool {
	if capability == "*" || capability == "admin" {
		return true
	}
	for _, action := range OperatorActions() {
		if capability == action {
			return true
		}
	}
	return false
}

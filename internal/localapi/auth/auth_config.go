package auth

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type Config struct {
	Token           string
	SubjectID       string
	Capabilities    []string
	ExpiresAt       time.Time
	TargetNode      string
	TargetPrincipal string
	Now             func() time.Time
}

func (c Config) validate() error {
	return c.Validate()
}

func (c Config) Validate() error {
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

func (c Config) subjectID() string {
	if strings.TrimSpace(c.SubjectID) != "" {
		return c.SubjectID
	}
	return "local-api"
}

func (c Config) capabilities() []string {
	return append([]string(nil), c.Capabilities...)
}

func (c Config) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func knownOperatorCapability(capability string) bool {
	if capability == "*" || capability == "admin" {
		return true
	}
	return slices.Contains(OperatorActions(), capability)
}

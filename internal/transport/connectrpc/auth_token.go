package connectrpc

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	identityapi "ardents/internal/identity/api"

	"connectrpc.com/connect"
)

func (c AuthConfig) callContext(header http.Header) (callContext, error) {
	token, ok := bearerToken(header.Get("Authorization"))
	if !ok || !secureTokenEqual(token, c.Token) {
		return callContext{}, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if !c.ExpiresAt.IsZero() && !c.now().Before(c.ExpiresAt) {
		return callContext{}, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("credential expired"))
	}
	if err := c.validateTargetBinding(header); err != nil {
		return callContext{}, err
	}
	capabilities, err := c.scopedCapabilities(header)
	if err != nil {
		return callContext{}, err
	}
	return identityapi.CallContext{
		Subject:       identityapi.SubjectRef{Kind: "token", ID: c.subjectID()},
		Capabilities:  capabilities,
		Authenticated: true,
	}, nil
}

func (c AuthConfig) validateTargetBinding(header http.Header) error {
	if err := matchTarget(header.Get(headerExpectedNode), c.TargetNode, "node"); err != nil {
		return err
	}
	return matchTarget(header.Get(headerExpectedPrincipal), c.TargetPrincipal, "principal")
}

func matchTarget(expected, actual, kind string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	if actual == "" || expected != actual {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("operator context %s binding mismatch", kind))
	}
	return nil
}

func (c AuthConfig) scopedCapabilities(header http.Header) ([]string, error) {
	requested := splitScopes(header.Get(headerScopes))
	if len(requested) == 0 {
		return c.capabilities(), nil
	}
	for _, scope := range requested {
		if !identityapi.HasActionCapability(c.Capabilities, scope) {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("operator context scope exceeds credential"))
		}
	}
	return requested, nil
}

func splitScopes(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func secureTokenEqual(provided, expected string) bool {
	providedDigest := sha256.Sum256([]byte(provided))
	expectedDigest := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(providedDigest[:], expectedDigest[:]) == 1
}

func bearerToken(headerValue string) (string, bool) {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return "", false
	}
	parts := strings.Fields(headerValue)
	if len(parts) != 2 || parts[0] != "Bearer" || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return parts[1], true
}

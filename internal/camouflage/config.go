package camouflage

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"net"
	"path"
	"strings"
)

const (
	candidateMagic   = "ardents-h3-wt1"
	candidateProfile = "webtunnel-v0.0.6"
	maximumEnvelope  = 1024
)

var errInvalidConfig = errors.New("adapter-config-invalid")

// Config is one immutable, validated WebTunnel adapter configuration. Its
// candidate-specific fields remain private to this package.
type Config struct {
	identity   [32]byte
	address    [4]byte
	port       uint16
	path       string
	serverName string
	chainPin   [32]byte
	commitment [32]byte
}

// Validate strictly decodes one signed candidate envelope for the selected
// Bridge identity. It performs no DNS, file, process, or network operation.
func Validate(raw []byte, identity [32]byte) (Config, error) {
	if len(raw) == 0 || len(raw) > maximumEnvelope {
		return Config{}, errInvalidConfig
	}
	reader := decoder{raw: raw}
	if !bytes.Equal(reader.take(len(candidateMagic)), []byte(candidateMagic)) || reader.byte() != 1 {
		return Config{}, errInvalidConfig
	}
	profile := reader.short(1, 63)
	address := reader.take(4)
	port := reader.uint16()
	httpsPath := reader.medium(1, 512)
	serverName := reader.short(1, 253)
	pin := reader.take(32)
	if reader.failed || !reader.done() || string(profile) != candidateProfile ||
		!validAddress(address) || port == 0 || !validPath(httpsPath) ||
		!validServerName(serverName) || allZero(pin) {
		return Config{}, errInvalidConfig
	}
	config := Config{identity: identity, port: port, path: string(httpsPath), serverName: string(serverName)}
	copy(config.address[:], address)
	copy(config.chainPin[:], pin)
	hash := sha256.New()
	hash.Write([]byte("ardents-h3-adapter-config-v1"))
	hash.Write([]byte{0})
	hash.Write(identity[:])
	hash.Write(raw)
	copy(config.commitment[:], hash.Sum(nil))
	return config, nil
}

// Commitment returns the candidate- and identity-bound validation commitment
// retained by transport-neutral Bridge state.
func (config Config) Commitment() [32]byte { return config.commitment }

func validAddress(raw []byte) bool {
	if len(raw) != net.IPv4len {
		return false
	}
	ip := net.IPv4(raw[0], raw[1], raw[2], raw[3])
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() &&
		!ip.IsLinkLocalUnicast() && !ip.IsUnspecified()
}

func validPath(raw []byte) bool {
	if len(raw) < 2 || raw[0] != '/' || bytes.ContainsAny(raw, "?#%") {
		return false
	}
	for _, value := range raw {
		if !pathByte(value) {
			return false
		}
	}
	value := string(raw)
	return path.Clean(value) == value && !strings.Contains(value, "//")
}

func pathByte(value byte) bool {
	if value == '/' || value == ':' || value == '@' || value == '-' || value == '.' || value == '_' || value == '~' {
		return true
	}
	if value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' {
		return true
	}
	return strings.ContainsRune("!$&'()*+,;=", rune(value))
}

func validServerName(raw []byte) bool {
	value := string(raw)
	if value == "" || strings.ToLower(value) != value || strings.HasSuffix(value, ".") || net.ParseIP(value) != nil {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if character != '-' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
				return false
			}
		}
	}
	return true
}

func allZero(raw []byte) bool {
	for _, value := range raw {
		if value != 0 {
			return false
		}
	}
	return true
}

package nativecircuit

import (
	"crypto/ecdh"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
)

func loadUserPlan(config roleConfig) (candidateUserPlan, error) {
	slot, err := parseRoleHandle(config.SlotHex)
	if err != nil {
		return candidateUserPlan{}, err
	}
	publicBytes, err := os.ReadFile(config.HPKEPublicPath)
	if err != nil {
		return candidateUserPlan{}, err
	}
	publicKey, err := ecdh.X25519().NewPublicKey(publicBytes)
	if err != nil {
		return candidateUserPlan{}, err
	}
	rootPEM, err := os.ReadFile(config.TargetRootPath)
	if err != nil {
		return candidateUserPlan{}, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootPEM) {
		return candidateUserPlan{}, errors.New("target root input is not a certificate")
	}
	leaf, err := parseDigest(config.ExpectedLeafSHA256)
	if err != nil {
		return candidateUserPlan{}, err
	}
	introductionPath, err := parseRolePath(config.IntroductionPath)
	if err != nil {
		return candidateUserPlan{}, err
	}
	dataPath, err := parseRolePath(config.DataPath)
	if err != nil {
		return candidateUserPlan{}, err
	}
	return candidateUserPlan{
		Profile: config.Profile, RunID: config.RunID, Rendezvous: config.Rendezvous, Slot: slot,
		IntroductionPath: introductionPath, DataPath: dataPath, HPKEPublic: publicKey,
		EndpointTrust: endpointTrust{Roots: roots, LeafSHA256: leaf}, Payload: seededPayload(config.PayloadSeed, config.PayloadBytes),
	}, nil
}

func loadServicePlan(config roleConfig) (candidateServicePlan, error) {
	slot, err := parseRoleHandle(config.SlotHex)
	if err != nil {
		return candidateServicePlan{}, err
	}
	privateBytes, err := os.ReadFile(config.HPKEPrivatePath)
	if err != nil {
		return candidateServicePlan{}, err
	}
	privateKey, err := ecdh.X25519().NewPrivateKey(privateBytes)
	if err != nil {
		return candidateServicePlan{}, err
	}
	certificate, err := tls.LoadX509KeyPair(config.EndpointCertificate, config.EndpointPrivateKey)
	if err != nil {
		return candidateServicePlan{}, err
	}
	introductionPath, err := parseRolePath(config.IntroductionPath)
	if err != nil {
		return candidateServicePlan{}, err
	}
	dataPath, err := parseRolePath(config.DataPath)
	if err != nil {
		return candidateServicePlan{}, err
	}
	return candidateServicePlan{
		Profile: config.Profile, RunID: config.RunID, Rendezvous: config.Rendezvous, Slot: slot,
		IntroductionPath: introductionPath, DataPath: dataPath, HPKEPrivate: privateKey, EndpointCertificate: certificate,
	}, nil
}

func parseRolePath(values []roleHop) ([]circuitHop, error) {
	result := make([]circuitHop, len(values))
	for index, value := range values {
		digest, err := parseDigest(value.CertificateSHA256)
		if err != nil || value.Address == "" {
			return nil, errors.New("native role path contains an invalid hop")
		}
		result[index] = circuitHop{Address: value.Address, CertificateSHA256: digest}
	}
	return result, nil
}

func parseDigest(value string) ([32]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return [32]byte{}, errors.New("SHA-256 identity must contain 32 bytes")
	}
	var result [32]byte
	copy(result[:], decoded)
	return result, nil
}

func parseRoleHandle(value string) (handle, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return handle{}, errors.New("native route handle must contain 32 bytes")
	}
	var result handle
	copy(result[:], decoded)
	return result, nil
}

func seededPayload(seed string, size int) []byte {
	result := make([]byte, size)
	var counter uint64
	for offset := 0; offset < len(result); counter++ {
		input := make([]byte, len(seed)+8)
		copy(input, seed)
		binary.BigEndian.PutUint64(input[len(seed):], counter)
		block := sha256.Sum256(input)
		offset += copy(result[offset:], block[:])
	}
	return result
}

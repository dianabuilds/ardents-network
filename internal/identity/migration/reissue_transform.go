package migration

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strings"

	discoveryrecords "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
)

type ReissueCounts struct {
	LocalReissued int `json:"local_reissued"`
	RemoteExpired int `json:"remote_expired"`
}

func reissueCapabilityLedger(ledger capabilityLedger, mappings map[string]string, issuerKeys map[string]ed25519.PublicKey, localLegacy string, localPrivate ed25519.PrivateKey) (capabilityLedger, ReissueCounts, error) {
	if len(localPrivate) != ed25519.PrivateKeySize || !bytes.Equal(localPrivate.Public().(ed25519.PublicKey), issuerKeys[localLegacy]) {
		return capabilityLedger{}, ReissueCounts{}, fmt.Errorf("local capability issuer key is invalid")
	}
	if _, ok := mappings[localLegacy]; !ok {
		return capabilityLedger{}, ReissueCounts{}, fmt.Errorf("local capability issuer has no verified mapping")
	}
	out := capabilityLedger{Grants: map[string]capabilityGrant{}, SenderGrants: map[string]capabilityGrant{}, Revocations: map[string]capabilityRevocation{}, DeliveryPrivateKey: append([]byte(nil), ledger.DeliveryPrivateKey...)}
	counts := ReissueCounts{}
	for _, collection := range []struct {
		source, target map[string]capabilityGrant
		kind           string
	}{{ledger.Grants, out.Grants, "grant"}, {ledger.SenderGrants, out.SenderGrants, "sender_grant"}} {
		if collection.source == nil {
			return capabilityLedger{}, ReissueCounts{}, fmt.Errorf("capability ledger is incomplete")
		}
		for _, key := range sortedKeys(collection.source) {
			grant := collection.source[key]
			artifact, err := inventoryCapabilityGrant(collection.kind, grant, mappings, issuerKeys, localLegacy)
			if err != nil {
				return capabilityLedger{}, ReissueCounts{}, err
			}
			if artifact.Classification != "reissue_signed_local_artifact" {
				counts.RemoteExpired++
				continue
			}
			grant.IssuerPrincipal = artifact.IssuerV1
			grant.SubjectPrincipal = artifact.SubjectV1
			grant.Signature = ed25519.Sign(localPrivate, capabilityDigest("ardents-capability-grant/1", canonicalCapabilityGrant(grant)))
			if !ed25519.Verify(localPrivate.Public().(ed25519.PublicKey), capabilityDigest("ardents-capability-grant/1", canonicalCapabilityGrant(grant)), grant.Signature) {
				return capabilityLedger{}, ReissueCounts{}, fmt.Errorf("reissued capability grant signature verification failed")
			}
			collection.target[key] = grant
			counts.LocalReissued++
		}
	}
	if ledger.Revocations == nil {
		return capabilityLedger{}, ReissueCounts{}, fmt.Errorf("capability ledger is incomplete")
	}
	for _, key := range sortedKeys(ledger.Revocations) {
		rev := ledger.Revocations[key]
		artifact, err := inventoryCapabilityRevocation(rev, mappings, issuerKeys, localLegacy)
		if err != nil {
			return capabilityLedger{}, ReissueCounts{}, err
		}
		if artifact.Classification != "reissue_signed_local_artifact" {
			counts.RemoteExpired++
			continue
		}
		rev.IssuerPrincipal = artifact.IssuerV1
		rev.Signature = ed25519.Sign(localPrivate, capabilityDigest("ardents-capability-revocation/1", canonicalCapabilityRevocation(rev)))
		if !ed25519.Verify(localPrivate.Public().(ed25519.PublicKey), capabilityDigest("ardents-capability-revocation/1", canonicalCapabilityRevocation(rev)), rev.Signature) {
			return capabilityLedger{}, ReissueCounts{}, fmt.Errorf("reissued capability revocation signature verification failed")
		}
		out.Revocations[key] = rev
		counts.LocalReissued++
	}
	return out, counts, nil
}

func reissueDiscoverySnapshot(snapshot discoverySnapshot, mappings map[string]string, localLegacy string, localPrivate ed25519.PrivateKey) (discoverySnapshot, ReissueCounts, error) {
	if len(localPrivate) != ed25519.PrivateKeySize {
		return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("local discovery signing key is invalid")
	}
	public := localPrivate.Public().(ed25519.PublicKey)
	legacy, err := LegacyPrincipalIDFromEd25519PublicKey(public)
	if err != nil || legacy.String() != localLegacy {
		return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("local discovery signing key does not match legacy Principal")
	}
	localV1, ok := mappings[localLegacy]
	if !ok {
		return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("local discovery Principal has no verified mapping")
	}
	derivedV1, err := identityprincipal.FromEd25519PublicKey(public)
	if err != nil || derivedV1.String() != localV1 {
		return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("local discovery mapping does not match signing key")
	}
	out := discoverySnapshot{Records: []discoveryrecords.Entry{}, State: snapshot.State, Reason: snapshot.Reason}
	counts := ReissueCounts{}
	for _, entry := range snapshot.Records {
		record := entry.Record
		publicRaw, decodeErr := base64.StdEncoding.Strict().DecodeString(record.PublicKey)
		signature, signatureErr := base64.StdEncoding.Strict().DecodeString(record.Signature)
		canonical, canonicalErr := discoveryrecords.Canonical(record)
		if decodeErr != nil || signatureErr != nil || canonicalErr != nil || len(publicRaw) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(publicRaw), canonical, signature) {
			return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("discovery record signature is invalid")
		}
		recordLegacy, deriveErr := LegacyPrincipalIDFromEd25519PublicKey(ed25519.PublicKey(publicRaw))
		if deriveErr != nil || recordLegacy.String() != record.Node {
			return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("discovery record Node does not match signing key")
		}
		mappedNode, mapped := mappings[record.Node]
		if !mapped {
			return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("discovery record Node has no verified mapping")
		}
		if entry.Source != discoveryrecords.Local {
			counts.RemoteExpired++
			continue
		}
		if record.Node != localLegacy || !bytes.Equal(publicRaw, public) {
			return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("local discovery record is not signed by the local Node")
		}
		record.Node = mappedNode
		if record.Owner != "" {
			mappedOwner, ownerOK := mappings[record.Owner]
			if !ownerOK {
				return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("discovery record owner has no verified mapping")
			}
			record.Owner = mappedOwner
		}
		if record.Kind == "node" {
			if record.Subject != localLegacy || record.ID != localLegacy+":node" {
				return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("discovery Node record has conflicting identity fields")
			}
			record.Subject, record.ID = localV1, localV1+":node"
		} else if strings.HasPrefix(record.Subject, "p_") {
			mappedSubject, subjectOK := mappings[record.Subject]
			if !subjectOK {
				return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("discovery record subject has no verified mapping")
			}
			record.Subject = mappedSubject
		}
		canonical, err = discoveryrecords.Canonical(record)
		if err != nil {
			return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("canonicalize reissued discovery record")
		}
		record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(localPrivate, canonical))
		decodedSignature, _ := base64.StdEncoding.Strict().DecodeString(record.Signature)
		if !ed25519.Verify(public, canonical, decodedSignature) {
			return discoverySnapshot{}, ReissueCounts{}, fmt.Errorf("reissued discovery signature verification failed")
		}
		entry.Record = record
		out.Records = append(out.Records, entry)
		counts.LocalReissued++
	}
	return out, counts, nil
}

func reissueRealmAuthority(state realmAuthorityState, mappings map[string]string) (realmAuthorityState, ed25519.PrivateKey, error) {
	if state.Version != "ardents.local-realm/v1" || !validRealmChannel(state.Discovery) || !validRealmChannel(state.Data) {
		return realmAuthorityState{}, nil, fmt.Errorf("local realm authority version is unsupported")
	}
	privateRaw, err := base64.StdEncoding.Strict().DecodeString(state.IssuerPrivate)
	if err != nil || len(privateRaw) != ed25519.PrivateKeySize {
		return realmAuthorityState{}, nil, fmt.Errorf("local realm authority key is invalid")
	}
	private := ed25519.PrivateKey(privateRaw)
	if !bytes.Equal(private, ed25519.NewKeyFromSeed(private.Seed())) {
		return realmAuthorityState{}, nil, fmt.Errorf("local realm authority key is invalid")
	}
	legacyIssuer, _ := LegacyPrincipalIDFromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	issuerV1, deriveErr := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if deriveErr != nil || mappings[legacyIssuer.String()] != issuerV1.String() {
		return realmAuthorityState{}, nil, fmt.Errorf("local realm issuer mapping is invalid")
	}
	members := make(map[string]realmNodeState, len(state.Members))
	for _, legacySubject := range sortedKeys(state.Members) {
		member := state.Members[legacySubject]
		mapped, ok := mappings[legacySubject]
		if !ok || member.Version != "ardents.local-realm-node/v1" || member.Subject != legacySubject || member.Issuer != legacyIssuer.String() || !validRealmGrant(member.Discovery) || !validRealmGrant(member.Data) {
			return realmAuthorityState{}, nil, fmt.Errorf("local realm member state is inconsistent")
		}
		member.Version, member.Subject, member.Issuer = "ardents.local-realm-node/v2", mapped, issuerV1.String()
		if _, duplicate := members[mapped]; duplicate {
			return realmAuthorityState{}, nil, fmt.Errorf("local realm member mapping collides")
		}
		members[mapped] = member
	}
	state.Version, state.Members = "ardents.local-realm/v2", members
	return state, private, nil
}

func reissueRealmNode(state realmNodeState, legacySubject, legacyIssuer string, mappings map[string]string) (realmNodeState, error) {
	if state.Version != "ardents.local-realm-node/v1" || state.Subject != legacySubject || state.Issuer != legacyIssuer || !validRealmGrant(state.Discovery) || !validRealmGrant(state.Data) {
		return realmNodeState{}, fmt.Errorf("local realm Node state is inconsistent")
	}
	subject, subjectOK := mappings[legacySubject]
	issuer, issuerOK := mappings[legacyIssuer]
	if !subjectOK || !issuerOK {
		return realmNodeState{}, fmt.Errorf("local realm Node mapping is incomplete")
	}
	state.Version, state.Subject, state.Issuer = "ardents.local-realm-node/v2", subject, issuer
	return state, nil
}

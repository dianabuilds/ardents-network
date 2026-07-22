package network

import (
	"errors"

	discoveryapi "ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

var ErrInvalidResourceTarget = errors.New("network resource target is invalid")

func CanonicalizeResource(procedure string, message any, kind identityaccess.ResourceKind) (identityaccess.ResourceTarget, error) {
	target := identityaccess.ResourceTarget{Kind: kind}
	valid := true
	switch procedure {
	case ardentsv1connect.NetworkServiceGetNetworkStatusProcedure:
		_, valid = message.(*protocol.GetNetworkStatusRequest)
	case ardentsv1connect.NetworkServiceGetDiscoveryStatusProcedure:
		_, valid = message.(*protocol.GetDiscoveryStatusRequest)
	case ardentsv1connect.NetworkServiceGetLocalPresenceProcedure:
		_, valid = message.(*protocol.GetLocalPresenceRequest)
	case ardentsv1connect.NetworkServiceListPeersProcedure:
		_, valid = message.(*protocol.ListPeersRequest)
	case ardentsv1connect.NetworkServiceListRouteCandidatesProcedure:
		_, valid = message.(*protocol.ListRouteCandidatesRequest)
	case ardentsv1connect.NetworkServiceListRecordsProcedure:
		_, valid = message.(*protocol.ListRecordsRequest)
	case ardentsv1connect.NetworkServiceResolveRecordProcedure:
		request, ok := message.(*protocol.ResolveRecordRequest)
		if !ok {
			valid = false
			break
		}
		var err error
		target.ID, err = discoveryrecord.AccessResourceID(request.GetKind(), request.GetSubject())
		valid = err == nil
	case ardentsv1connect.NetworkServiceResolveServiceProcedure:
		request, ok := message.(*protocol.ResolveServiceRequest)
		if !ok {
			valid = false
			break
		}
		var err error
		target.ID, err = discoveryapi.ServiceAccessResourceID(request.GetService())
		valid = err == nil
	case ardentsv1connect.NetworkServiceImportRecordProcedure:
		request, ok := message.(*protocol.ImportRecordRequest)
		if !ok || request.GetRecord() == nil {
			valid = false
			break
		}
		snapshot := fromDiscoveryRecord(request.GetRecord())
		record := discoveryapi.RecordFromSnapshot(snapshot)
		valid = discoveryrecord.ValidAccessIdentifier(record.ID) && discoveryrecord.Validate(record) == nil
		target.ID = record.ID
	default:
		valid = false
	}
	if !valid {
		return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
	}
	return target, nil
}

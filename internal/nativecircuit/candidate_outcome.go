package nativecircuit

import (
	"errors"
	"io"
	"net"
)

var errCandidateContractFailure = errors.New("native candidate contract assertion")
var errCandidateDownstreamFailure = errors.New("native candidate downstream peer failure")

func candidateContractFailure(message string) error {
	return errors.Join(errCandidateContractFailure, errors.New(message))
}

func candidatePeerReadFailure(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return errors.Join(errCandidateDownstreamFailure, err)
	}
	return err
}

func candidateFailureKind(err error) string {
	if errors.Is(err, errCandidateContractFailure) {
		return "scenario"
	}
	if errors.Is(err, errCandidateDownstreamFailure) {
		return "downstream"
	}
	return ""
}

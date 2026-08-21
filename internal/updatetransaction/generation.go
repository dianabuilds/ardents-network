package updatetransaction

import (
	"encoding/binary"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dianabuilds/ardents-network/internal/releasedecision"
)

type manifestView struct {
	Generation                            uint64
	TargetPath, SafeNotice, CustodyNotice string
	Length                                uint64
	Artifact                              [32]byte
}
type currentSelection struct {
	Transaction uint64
	Current     inspectedTuple
	Rollback    *inspectedTuple
}
type bodyWriter struct {
	body []byte
	err  error
}
type rootInspection struct {
	selection   currentSelection
	predecessor predecessorInspection
}

func canonicalUnixSeconds(value time.Time) uint64 {
	if value.IsZero() {
		return 0
	}
	return uint64(value.Unix())
}
func (writer *bodyWriter) number(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.body = append(writer.body, encoded[:]...)
}
func (writer *bodyWriter) text(value string, maximum int) {
	if writer.err != nil {
		return
	}
	if !utf8.ValidString(value) || len(value) > maximum || len(value) > int(^uint16(0)) {
		writer.err = errRecordInvalid
		return
	}
	if strings.IndexByte(value, 0) >= 0 {
		writer.err = errRecordInvalid
		return
	}
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(value)))
	writer.body = append(writer.body, length[:]...)
	writer.body = append(writer.body, value...)
}
func encodeManifest(request Request, artifact [32]byte) ([]byte, error) {
	decision := request.Decision
	if !completeFloors(decision.Floors) {
		return nil, errRecordInvalid
	}
	writer := &bodyWriter{}
	writer.number(request.Generation)
	writer.text(decision.Path, maximumTargetBytes)
	writer.number(uint64(decision.Length))
	writer.body = append(writer.body, artifact[:]...)
	for _, value := range []string{decision.Platform, decision.Architecture,
		decision.Environment, decision.Network, decision.ReleaseIdentity} {
		writer.text(value, maximumIdentityBytes)
	}
	writer.number(uint64(decision.ReleaseVersion))
	for _, value := range []string{decision.SourceRevision, decision.BuildInputCommitment,
		decision.BuildIdentity, decision.DependencyIdentity, decision.SBOMIdentity,
		decision.AttestationPolicy, decision.Qualification, decision.BuildState,
		decision.ProtocolPhase, string(decision.BuildSafety), string(decision.Protocol)} {
		writer.text(value, maximumIdentityBytes)
	}
	floors := decision.Floors
	for _, floor := range []struct {
		version int64
		digest  []byte
	}{{floors.RootVersion, floors.RootDigest}, {floors.TimestampVersion, floors.TimestampDigest},
		{floors.SnapshotVersion, floors.SnapshotDigest}, {floors.TargetsVersion, floors.TargetsDigest}} {
		writer.number(uint64(floor.version))
		writer.body = append(writer.body, floor.digest...)
	}
	for _, value := range []int64{decision.ReferenceTime.Unix(),
		decision.BuildSafetyNoNewWorkAfter.Unix(), decision.BuildSafetyTerminateAfter.Unix()} {
		writer.number(uint64(value))
	}
	writer.number(canonicalUnixSeconds(decision.ProtocolTransitionDeadline))
	writer.text(request.SchemaPlan, maximumIdentityBytes)
	writer.text("update committed", maximumNoticeBytes)
	writer.text(decision.CustodyNotice, maximumNoticeBytes)
	for _, value := range []string{string(decision.Outcome), decision.Platform,
		decision.Architecture, decision.Environment, decision.Network} {
		writer.text(value, maximumIdentityBytes)
	}
	writer.body = append(writer.body, 1, 0, 1)
	if writer.err != nil {
		return nil, writer.err
	}
	return encodeRecord(recordManifest, writer.body, maximumRecordBytes)
}
func completeFloors(floors releasedecision.FloorSet) bool {
	return floors.RootVersion > 0 && len(floors.RootDigest) == 32 && floors.TimestampVersion > 0 && len(floors.TimestampDigest) == 32 &&
		floors.SnapshotVersion > 0 && len(floors.SnapshotDigest) == 32 && floors.TargetsVersion > 0 && len(floors.TargetsDigest) == 32
}

type bodyReader struct {
	body   []byte
	offset int
	err    error
}

func (reader *bodyReader) take(length int) []byte {
	if reader.err != nil || length < 0 || length > len(reader.body)-reader.offset {
		reader.err = errRecordInvalid
		return nil
	}
	value := reader.body[reader.offset : reader.offset+length]
	reader.offset += length
	return value
}
func (reader *bodyReader) number() uint64 {
	value := reader.take(8)
	if value == nil {
		return 0
	}
	return binary.BigEndian.Uint64(value)
}
func (reader *bodyReader) text(maximum int) string {
	length := reader.take(2)
	if length == nil {
		return ""
	}
	value := reader.take(int(binary.BigEndian.Uint16(length)))
	if len(value) > maximum || !utf8.Valid(value) {
		reader.err = errRecordInvalid
	}
	if strings.IndexByte(string(value), 0) >= 0 {
		reader.err = errRecordInvalid
	}
	return string(value)
}
func (reader *bodyReader) digest() [32]byte {
	var digest [32]byte
	copy(digest[:], reader.take(32))
	return digest
}
func (reader *bodyReader) boolean() bool {
	value := reader.take(1)
	if value == nil || value[0] > 1 {
		reader.err = errRecordInvalid
		return false
	}
	return value[0] == 1
}
func (reader *bodyReader) done() error {
	if reader.err != nil || reader.offset != len(reader.body) {
		return errRecordInvalid
	}
	return nil
}
func decodeManifest(raw []byte) (manifestView, error) {
	var view manifestView
	body, err := decodeRecord(raw, recordManifest, maximumRecordBytes)
	if err != nil {
		return view, err
	}
	reader := &bodyReader{body: body}
	view.Generation, view.TargetPath = reader.number(), reader.text(maximumTargetBytes)
	view.Length, view.Artifact = reader.number(), reader.digest()
	identity := make([]string, 5)
	for index := range identity {
		identity[index] = reader.text(maximumIdentityBytes)
	}
	reader.number()
	for range 11 {
		reader.text(maximumIdentityBytes)
	}
	for range 4 {
		if reader.number() == 0 {
			reader.err = errRecordInvalid
		}
		reader.digest()
	}
	for range 4 {
		reader.number()
	}
	if reader.text(maximumIdentityBytes) != "no-op-v1" {
		reader.err = errRecordInvalid
	}
	view.SafeNotice = reader.text(maximumNoticeBytes)
	view.CustodyNotice = reader.text(maximumNoticeBytes)
	authorization := make([]string, 5)
	for index := range authorization {
		authorization[index] = reader.text(maximumIdentityBytes)
	}
	if authorization[0] != "release-accepted" || authorization[1] != identity[0] ||
		authorization[2] != identity[1] || authorization[3] != identity[2] ||
		authorization[4] != identity[3] || !reader.boolean() || reader.boolean() ||
		!reader.boolean() {
		reader.err = errRecordInvalid
	}
	return view, reader.done()
}

func encodeCurrent(selection currentSelection) ([]byte, error) {
	writer := &bodyWriter{}
	writer.number(selection.Transaction)
	appendCurrentTuple(writer, selection.Current)
	writer.body = append(writer.body, 0)
	if selection.Rollback != nil {
		writer.body[len(writer.body)-1] = 1
		appendCurrentTuple(writer, *selection.Rollback)
	}
	return encodeRecord(recordCurrent, writer.body, maximumRecordBytes)
}

func appendCurrentTuple(writer *bodyWriter, tuple inspectedTuple) {
	writer.number(tuple.Generation)
	writer.number(tuple.Length)
	writer.body = append(writer.body, tuple.Artifact[:]...)
	writer.body = append(writer.body, tuple.Manifest[:]...)
}

func decodeCurrent(raw []byte) (currentSelection, error) {
	var selection currentSelection
	body, err := decodeRecord(raw, recordCurrent, maximumRecordBytes)
	if err != nil {
		return selection, err
	}
	reader := &bodyReader{body: body}
	selection.Transaction = reader.number()
	selection.Current = readCurrentTuple(reader)
	if reader.boolean() {
		rollback := readCurrentTuple(reader)
		selection.Rollback = &rollback
	}
	return selection, reader.done()
}

func readCurrentTuple(reader *bodyReader) inspectedTuple {
	return inspectedTuple{Generation: reader.number(), Length: reader.number(),
		Artifact: reader.digest(), Manifest: reader.digest()}
}

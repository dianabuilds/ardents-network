package infrastructure

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"syscall"
)

// Private original historical completion, never a live caller or permission.
// Its independently opened native journal/result lineage and absent guard are
// required in addition to this codec. Fresh Release/State/manager admission is
// still owned by the later recovery operation.
type sourceCutResult struct {
	Schema        string                  `json:"schema"`
	Boundary      sourceTransitionMessage `json:"boundary"`
	Origin        sourceTransitionOrigin  `json:"origin"`
	ControlDevice uint64                  `json:"control_device"`
	ControlInode  uint64                  `json:"control_inode"`
	Acknowledged  uint32                  `json:"acknowledged"`
	Code          int                     `json:"code"`
	Status        int                     `json:"status"`
	Reaped        int                     `json:"reaped"`
}

func sourceCutPaths(journal string) (control, result, pending string) {
	return journal + ".original-cut-journal", journal + ".original-cut-result", journal + ".original-cut-pending"
}

func sourceCutPending(message sourceTransitionMessage, origin sourceTransitionOrigin) []byte {
	raw, _ := json.Marshal(struct {
		Boundary sourceTransitionMessage `json:"boundary"`
		Origin   sourceTransitionOrigin  `json:"origin"`
	}{message, origin})
	return []byte("ardents-source-original-cut-pending-v1\n" + digest(raw) + "\n")
}

func decodeSourceCutResult(controlRaw, resultRaw, prefixRaw []byte, controlInfo, resultInfo, originalInfo os.FileInfo, journal, operation, generation, binding, program string) (sourceCutResult, error) {
	var result sourceCutResult
	if len(resultRaw) == 0 || len(resultRaw) > 64<<10 || resultRaw[len(resultRaw)-1] != '\n' || json.Unmarshal(resultRaw, &result) != nil {
		return result, ErrBinding
	}
	canonical, err := json.Marshal(result)
	if err != nil || !bytes.Equal(resultRaw, append(canonical, '\n')) || !sourceAccess(controlInfo, 0600, 0, 0, false) || !sourceAccess(resultInfo, 0600, 0, 0, false) || !sourceAccess(originalInfo, 0600, 0, 0, false) || controlInfo.Size() != int64(len(controlRaw)) || resultInfo.Size() != int64(len(resultRaw)) {
		return result, ErrBinding
	}
	controlPath, resultPath, pending := sourceCutPaths(journal)
	c, r, o := controlInfo.Sys().(*syscall.Stat_t), resultInfo.Sys().(*syscall.Stat_t), originalInfo.Sys().(*syscall.Stat_t)
	b := result.Boundary
	p := result.Origin.Process
	if len(b.ManagerIdentity) > 288 {
		return result, ErrBinding
	}
	if result.Schema != "ardents-source-original-cut-v1" || result.ControlDevice != uint64(c.Dev) || result.ControlInode != c.Ino || result.Acknowledged+1 != b.Sequence || result.Code != 2 || result.Status != int(syscall.SIGKILL) || result.Reaped != 1 ||
		b.Schema != "ardents-source-transition-boundary-v1" || !sourceInitialSHA(b.Nonce) || b.Deadline != 0 || b.Sequence == 0 || b.Records != int(b.Sequence)+1 || b.Records > 115 || b.Journal != journal || b.Operation != operation || b.Generation != generation || b.BindingSHA256 != binding || b.Device != uint64(o.Dev) || b.Inode != o.Ino || b.PrefixSHA256 != digest(prefixRaw) || int64(len(prefixRaw)) != originalInfo.Size() ||
		!sourceInitialSHA(program) || result.Origin.ProgramSHA256 != program || result.Origin.ProgramDevice == 0 || result.Origin.ProgramInode == 0 || result.Origin.NativeManager == "" || len(result.Origin.NativeManager) > 4096 || p.PID <= 1 || p.Parent <= 1 || p.PID == p.Parent || p.Device == 0 || p.Inode == 0 || p.Started == 0 {
		return result, ErrBinding
	}
	prefix, err := decodeSourceRecoveryPrefix(prefixRaw, 115)
	if err != nil || len(prefix) != b.Records || prefix[0].Path != journal || prefix[0].Device != b.Device || prefix[0].Inode != b.Inode || sourceRecoveryCompleteCount(operation, prefix[0].Phase) == 0 {
		return result, ErrBinding
	}
	records, err := decodeSourceCanonicalPrefix(controlRaw, 13)
	if err != nil || len(records) != 13 {
		return result, ErrBinding
	}
	pendingSHA := digest(sourceCutPending(b, result.Origin))
	boundaryRaw, _ := json.Marshal(b)
	sha := digest(resultRaw)
	pidPath := "/proc/" + strconv.FormatUint(uint64(p.PID), 10)
	wanted := []sourceRecord{
		{Phase: "source-cut", Path: controlPath, Device: uint64(c.Dev), Inode: c.Ino, Mode: 0600},
		{Phase: "file-intent", Path: pending, SHA256: pendingSHA, Mode: 0600},
		{Phase: "file-born", Path: pending, SHA256: pendingSHA, Mode: 0600},
		{Phase: "file-ready", Path: pending, SHA256: pendingSHA, Mode: 0600},
		{Phase: "coordinator-born-observed", Path: pidPath, Device: p.Device, Inode: p.Inode, SHA256: digest(resultRaw)},
		{Phase: "boundary-held", Path: journal, Device: b.Device, Inode: b.Inode, SHA256: digest(boundaryRaw)},
		{Phase: "next-effect-denied", Path: journal, Device: b.Device, Inode: b.Inode, SHA256: digest(boundaryRaw)},
		{Phase: "native-joined", Path: pidPath, Device: p.Device, Inode: p.Inode, SHA256: sha},
		{Phase: "physical-closed", Path: journal, Device: b.Device, Inode: b.Inode, SHA256: sha},
		{Phase: "file-intent", Path: resultPath, SHA256: sha, Mode: 0600},
		{Phase: "file-born", Path: resultPath, Device: uint64(r.Dev), Inode: r.Ino, SHA256: sha, Mode: 0600},
		{Phase: "file-ready", Path: resultPath, Device: uint64(r.Dev), Inode: r.Ino, SHA256: sha, Mode: 0600},
		{Phase: "original-cut-completed", Path: resultPath, Device: uint64(r.Dev), Inode: r.Ino, SHA256: sha, Mode: 0600},
	}
	if records[2].Device == 0 || records[2].Inode == 0 || records[2].Device != records[3].Device || records[2].Inode != records[3].Inode {
		return result, ErrBinding
	}
	wanted[2].Device, wanted[2].Inode = records[2].Device, records[2].Inode
	wanted[3].Device, wanted[3].Inode = records[3].Device, records[3].Inode
	for i := range records {
		wanted[i].Sequence = i
		if records[i] != wanted[i] {
			return result, ErrBinding
		}
	}
	return result, nil
}

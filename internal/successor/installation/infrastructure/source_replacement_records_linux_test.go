package infrastructure

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// This byte oracle encodes the closed record fields independently. Supplied
// identities isolate the transcript rule; they provide no physical birth,
// successful retirement, Release authorization or installed qualification.
func sourceReplacementRecordVector(t *testing.T) ([]byte, sourceReplacementBinding, map[string]string) {
	t.Helper()
	raw := sourceReplacementBindingVector()
	b, err := decodeSourceReplacementBinding(raw)
	if err != nil {
		t.Fatal(err)
	}
	root := "/var/lib/install"
	sealed := root + "/generations/" + strings.Repeat("a", 64)
	hashes := map[string]string{root + "/binding.json": digest(raw), "/etc/systemd/system/ardents-source.service": b.Configuration.UnitSHA256}
	var lines []string
	add := func(phase, path string, inode uint64, sha string, mode, uid, gid uint32) {
		device := uint64(0)
		if inode != 0 {
			device = 1
		}
		lines = append(lines, fmt.Sprintf(`{"sequence":%d,"phase":%q,"path":%q,"device":%d,"inode":%d,"sha256":%q,"mode":%d,"uid":%d,"gid":%d}`, len(lines), phase, path, device, inode, sha, mode, uid, gid))
	}
	add("initial", root+".source-replacement-"+b.Configuration.Generation+"-journal", 1, "", 0600, 0, 0)
	add("replacement-admitted", root, 0, strings.Repeat("3", 64), 0, 999, 999)
	add("predecessor-binding", root+"/binding.json", 2, b.PredecessorBindingSHA256, 0644, 0, 0)
	add("predecessor-unit", "/etc/systemd/system/ardents-source.service", 3, strings.Repeat("4", 64), 0644, 0, 0)
	add("predecessor-retired", root+"/source-startup/source-retirement.json", 4, b.RetirementSHA256, 0600, 0, 0)
	write := func(path, sha string, mode uint32, inode uint64) {
		add("file-intent", path, 0, sha, mode, 0, 0)
		add("file-born", path, inode, sha, mode, 0, 0)
		add("file-ready", path, inode, sha, mode, 0, 0)
	}
	write(root+"/source-replacement-pending", digest([]byte("ardents-source-replacement-pending-v1\n"+hashes[root+"/binding.json"]+"\n")), 0600, 5)
	add("directory-intent", sealed, 0, "", 0755, 0, 0)
	add("directory-born", sealed, 6, "", 0755, 0, 0)
	add("directory-ready", sealed, 6, "", 0755, 0, 0)
	for i, name := range []string{"ardents-node-linux-amd64", "ardents-source.service", "binding.json", "infrastructure-enrollment.json", "infrastructure-source.json", "rendered-source.service", "root.json", "snapshot.json", "source-plan.json", "source-request.json", "targets.json", "timestamp.json"} {
		mode := uint32(0444)
		if i == 0 {
			mode = 0555
		}
		path := filepath.Join(sealed, name)
		hashes[path] = digest([]byte(name))
		write(path, hashes[path], mode, uint64(10+i))
	}
	add("seal-intent", sealed, 6, b.Configuration.Generation, 0555, 0, 0)
	add("sealed", sealed, 6, b.Configuration.Generation, 0555, 0, 0)
	add("selection-intent", root+"/binding.json", 2, hashes[root+"/binding.json"], 0644, 0, 0)
	add("selection-written", root+"/binding.json", 2, hashes[root+"/binding.json"], 0644, 0, 0)
	add("unit-intent", "/etc/systemd/system/ardents-source.service", 3, hashes["/etc/systemd/system/ardents-source.service"], 0644, 0, 0)
	add("unit-written", "/etc/systemd/system/ardents-source.service", 3, hashes["/etc/systemd/system/ardents-source.service"], 0644, 0, 0)
	add("reload-intent", "/etc/systemd/system/ardents-source.service", 0, hashes["/etc/systemd/system/ardents-source.service"], 0644, 0, 0)
	add("replacement-verified", root+"/binding.json", 0, hashes[root+"/binding.json"], 0644, 0, 0)
	return []byte(strings.Join(lines, "\n") + "\n"), b, hashes
}

func TestSourceReplacementRecordsRequireCompleteSameInodeTransition(t *testing.T) {
	raw, b, hashes := sourceReplacementRecordVector(t)
	read := func(raw []byte) (map[string]sourceRecord, error) {
		return decodeSourceReplacementRecords(raw, b, hashes, strings.Repeat("3", 64), strings.Repeat("4", 64))
	}
	owned, err := read(raw)
	if err != nil || owned["/var/lib/install/binding.json"].Inode != 2 || owned["/etc/systemd/system/ardents-source.service"].Inode != 3 {
		t.Fatal("independent replacement transcript", err)
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	for n := 0; n < len(lines); n++ {
		if _, err := read(bytes.Join(lines[:n], []byte{'\n'})); err == nil {
			t.Fatal("partial transcript accepted", n)
		}
	}
	for _, field := range []string{"sequence", "phase", "path", "device", "inode", "sha256", "mode", "uid", "gid"} {
		altered := bytes.Replace(raw, []byte(`"`+field+`":`), []byte(`"unknown_`+field+`":`), 1)
		if _, err := read(altered); err == nil {
			t.Fatal("missing/unknown field accepted", field)
		}
	}
	for _, phase := range []string{"selection-intent", "selection-written", "unit-intent", "unit-written"} {
		for i, line := range lines {
			if !bytes.Contains(line, []byte(`"phase":"`+phase+`"`)) {
				continue
			}
			copyLines := append([][]byte(nil), lines...)
			copyLines[i] = bytes.Replace(line, []byte(`"device":1`), []byte(`"device":2`), 1)
			if _, err := read(append(bytes.Join(copyLines, []byte{'\n'}), '\n')); err == nil {
				t.Fatal("foreign publication inode accepted", phase)
			}
		}
	}
}

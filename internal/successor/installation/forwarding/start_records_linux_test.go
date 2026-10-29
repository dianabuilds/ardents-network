package forwarding

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// These detached literal transcripts test only the closed persisted grammar;
// no fake process, peer, manager or successful installed authority is supplied.
func startRecordVector() ([]byte, []string) {
	root, journal := "/opt/forwarding/forwarding-startup", "/opt/forwarding.forwarding-first-start-journal"
	hashes := []string{strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64)}
	sum := sha256.Sum256([]byte("ardents-forwarding-start-pending-v1\n" + hashes[0] + "\n"))
	pending := hex.EncodeToString(sum[:])
	type item struct {
		phase, path, hash string
		mode, gid         uint32
	}
	items := []item{
		{"first-start", journal, hashes[0], 0600, 0},
		{"directory-intent", root, "", 0, 0}, {"directory-born", root, "", 0700, 0}, {"directory-ready", root, "", 0750, 988},
		{"file-intent", root + "/forwarding-start-pending", pending, 0, 0}, {"file-born", root + "/forwarding-start-pending", pending, 0600, 0}, {"file-ready", root + "/forwarding-start-pending", pending, 0600, 0},
		{"file-intent", root + "/start-guard.json", hashes[1], 0, 0}, {"file-born", root + "/start-guard.json", hashes[1], 0600, 0}, {"file-ready", root + "/start-guard.json", hashes[1], 0600, 0},
		{"socket-intent", root + "/start-completion.socket", "", 0, 0}, {"socket-born", root + "/start-completion.socket", "", 0755, 0}, {"socket-ready", root + "/start-completion.socket", "", 0660, 988},
		{"file-intent", root + "/start-socket.json", hashes[2], 0, 0}, {"file-born", root + "/start-socket.json", hashes[2], 0600, 0}, {"file-ready", root + "/start-socket.json", hashes[2], 0600, 0},
		{"start-attempt", root, hashes[1], 0, 0}, {"invocation-observed", root, hashes[1], 0, 0}, {"completion-received", root, hashes[4], 0, 0},
		{"file-intent", root + "/start-archived.json", hashes[3], 0, 0}, {"file-born", root + "/start-archived.json", hashes[3], 0600, 0}, {"file-ready", root + "/start-archived.json", hashes[3], 0600, 0},
		{"archived", root + "/start-archived.json", hashes[3], 0600, 0}, {"ack-attempt", root, hashes[4], 0, 0}, {"ack-written", root, hashes[4], 0, 0},
	}
	var raw []byte
	inodes := map[string]uint64{}
	for i, item := range items {
		var device, inode uint64
		if item.mode != 0 {
			device = 5
			inode = inodes[item.path]
			if inode == 0 {
				inode = uint64(len(inodes) + 10)
				inodes[item.path] = inode
			}
		}
		raw = fmt.Appendf(raw, "{\"schema\":\"ardents-forwarding-start-journal-v1\",\"sequence\":%d,\"phase\":%q,\"path\":%q,\"device\":%d,\"inode\":%d,\"sha256\":%q,\"mode\":%d,\"uid\":0,\"gid\":%d}\n", i, item.phase, item.path, device, inode, item.hash, item.mode, item.gid)
	}
	return raw, hashes
}

func TestForwardingStartContextDoesNotMakeLegacyOrPartialHistoryContinuable(t *testing.T) {
	legacy, hashes := startRecordVector()
	lines := bytes.Split(legacy[:len(legacy)-1], []byte{'\n'})
	native := fmt.Appendf(nil, "{\"schema\":\"ardents-forwarding-start-journal-v1\",\"sequence\":1,\"phase\":\"native-context\",\"path\":\"/opt/forwarding\",\"device\":0,\"inode\":0,\"sha256\":%q,\"mode\":0,\"uid\":0,\"gid\":0}", strings.Repeat("f", 64))
	current := [][]byte{bytes.Clone(lines[0]), native}
	for n, line := range lines[1:] {
		current = append(current, bytes.Replace(line, fmt.Appendf(nil, "\"sequence\":%d,", n+1), fmt.Appendf(nil, "\"sequence\":%d,", n+2), 1))
	}
	encode := func(lines [][]byte) []byte { return append(bytes.Join(lines, []byte{'\n'}), '\n') }
	decode := func(raw []byte) error {
		_, err := decodeStartRecords(raw, 0, "/opt/forwarding.forwarding-first-start-journal", "/opt/forwarding/forwarding-startup", hashes[0], hashes[1], hashes[2], hashes[3], hashes[4], 988)
		return err
	}
	if err := decode(legacy); err != nil {
		t.Fatal("complete historical25-record grammar", err)
	}
	if err := decode(encode(current)); err != nil {
		t.Fatal("complete26-record context grammar", err)
	}
	for n := 0; n < len(current); n++ {
		if decode(encode(current[:n])) == nil {
			t.Fatal("new prefix became complete historical start", n)
		}
	}
	for _, fault := range []string{"path", "phase", "device", "inode", "mode", "uid", "gid", "hash", "schema", "sequence"} {
		var changed startEntry
		if err := json.Unmarshal(native, &changed); err != nil {
			t.Fatal(err)
		}
		switch fault {
		case "path":
			changed.Path += "-foreign"
		case "phase":
			changed.Phase = "context-ready"
		case "device":
			changed.Device = 1
		case "inode":
			changed.Inode = 1
		case "mode":
			changed.Mode = 0600
		case "uid":
			changed.UID = 988
		case "gid":
			changed.GID = 988
		case "hash":
			changed.SHA256 = "unknown"
		case "schema":
			changed.Schema = "ardents-source-start-journal-v1"
		case "sequence":
			changed.Sequence = 2
		}
		raw, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		bad := append([][]byte(nil), current...)
		bad[1] = raw
		if decode(encode(bad)) == nil {
			t.Fatal("foreign native context record", fault)
		}
	}
	// Neither grammar decoder supplies an original-cut proof, native readback,
	// completed Sync/Close, Release authority or a recovery continuation right.
}

func TestForwardingStartRecordsRequireWholeOriginalOrderedTranscript(t *testing.T) {
	raw, hashes := startRecordVector()
	decode := func(body []byte) error {
		_, err := decodeStartRecords(body, 0, "/opt/forwarding.forwarding-first-start-journal", "/opt/forwarding/forwarding-startup", hashes[0], hashes[1], hashes[2], hashes[3], hashes[4], 988)
		return err
	}
	if err := decode(raw); err != nil {
		t.Fatal("independent full byte vector refused", err)
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 25 {
		t.Fatal("independent transcript length differs")
	}
	for i := range lines {
		missing := append(bytes.Join(append(append([][]byte(nil), lines[:i]...), lines[i+1:]...), []byte{'\n'}), '\n')
		if decode(missing) == nil {
			t.Fatal("partial transcript admitted", i)
		}
		changed := append([][]byte(nil), lines...)
		changed[i] = bytes.Replace(lines[i], []byte("ardents-forwarding-start-journal-v1"), []byte("ardents-source-first-start-journal-v1"), 1)
		if decode(append(bytes.Join(changed, []byte{'\n'}), '\n')) == nil {
			t.Fatal("foreign journal phase admitted", i)
		}
		if i > 0 {
			changed = append([][]byte(nil), lines...)
			changed[i-1], changed[i] = changed[i], changed[i-1]
			if decode(append(bytes.Join(changed, []byte{'\n'}), '\n')) == nil {
				t.Fatal("reordered phase admitted", i)
			}
		}
	}
	for _, changed := range [][]byte{
		bytes.Replace(raw, []byte(`"inode":10`), []byte(`"inode":0`), 1),
		bytes.Replace(raw, []byte(`"mode":384`), []byte(`"mode":420`), 1),
		bytes.Replace(raw, []byte(`"gid":988`), []byte(`"gid":989`), 1),
		bytes.Replace(raw, []byte(`"sequence":1,`), []byte(`"sequence":0,`), 1),
		bytes.Replace(raw, []byte(`"uid":0`), []byte(`"uid":988`), 1),
		bytes.Replace(raw, []byte(`"phase":"first-start"`), []byte(`"phase":"invocation"`), 1),
		append(bytes.Clone(raw), '\n'), raw[:len(raw)-1],
	} {
		if decode(changed) == nil {
			t.Fatal("ambiguous/foreign transcript admitted")
		}
	}
}

func TestForwardingStartRecordsKeepRequestAndNewACKCommitmentsDistinct(t *testing.T) {
	raw, hashes := startRecordVector()
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	ack := digest([]byte("independent new full 161-byte ACK commitment"))
	for _, index := range []int{len(lines) - 2, len(lines) - 1} {
		var record startEntry
		if json.Unmarshal(lines[index], &record) != nil || record.Phase != "ack-attempt" && record.Phase != "ack-written" {
			t.Fatal("independent literal's ACK positions changed")
		}
		record.SHA256 = ack
		lines[index], _ = json.Marshal(record)
	}
	body := append(bytes.Join(lines, []byte{'\n'}), '\n')
	decode := func(request, response string) error {
		_, err := decodeStartRecordsFrames(body, 0, "/opt/forwarding.forwarding-first-start-journal", "/opt/forwarding/forwarding-startup", hashes[0], hashes[1], hashes[2], hashes[3], request, response, 988)
		return err
	}
	if err := decode(hashes[4], ack); err != nil {
		t.Fatal("new full ACK commitment unavailable", err)
	}
	if decode(hashes[4], hashes[4]) == nil || decode(ack, ack) == nil {
		t.Fatal("request and full ACK became interchangeable")
	}
}

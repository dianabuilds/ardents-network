package forwarding

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func retirementRecordVector() ([]byte, []string) {
	directory, journal := "/opt/f/forwarding-retirement-00000000", "/opt/f.forwarding-retirement-00000000-journal"
	hashes := []string{strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64)}
	sum := sha256.Sum256([]byte("ardents-forwarding-retirement-pending-v1\n" + hashes[0] + "\n"))
	pending := hex.EncodeToString(sum[:])
	type item struct {
		phase, path, hash string
		mode, gid         uint32
	}
	items := []item{
		{"stop", journal, hashes[0], 0600, 0},
		{"directory-intent", directory, "", 0, 0}, {"directory-born", directory, "", 0700, 0}, {"directory-ready", directory, "", 0750, 988},
		{"file-intent", directory + "/retirement-pending", pending, 0, 0}, {"file-born", directory + "/retirement-pending", pending, 0600, 0}, {"file-ready", directory + "/retirement-pending", pending, 0600, 0},
		{"socket-intent", directory + "/stop-completion.socket", "", 0, 0}, {"socket-born", directory + "/stop-completion.socket", "", 0755, 0}, {"socket-ready", directory + "/stop-completion.socket", "", 0660, 988},
		{"interrupt-attempt", directory, hashes[1], 0, 0}, {"domain-joined", directory, hashes[2], 0, 0}, {"native-joined", directory, hashes[3], 0, 0}, {"native-closed", directory, hashes[3], 0, 0},
		{"file-intent", directory + "/retirement-result.json", hashes[3], 0, 0}, {"file-born", directory + "/retirement-result.json", hashes[3], 0600, 0}, {"file-ready", directory + "/retirement-result.json", hashes[3], 0600, 0}, {"retired", directory + "/retirement-result.json", hashes[3], 0600, 0},
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
		raw = fmt.Appendf(raw, "{\"schema\":\"ardents-forwarding-retirement-journal-v1\",\"sequence\":%d,\"phase\":%q,\"path\":%q,\"device\":%d,\"inode\":%d,\"sha256\":%q,\"mode\":%d,\"uid\":0,\"gid\":%d}\n", i, item.phase, item.path, device, inode, item.hash, item.mode, item.gid)
	}
	return raw, hashes
}

// Detached bytes test only the persisted grammar; they supply no domain/native
// completion or restart authority and are never passed to a live root owner.
func TestForwardingRetirementRecordsRequireEveryOriginalClosePhase(t *testing.T) {
	raw, hashes := retirementRecordVector()
	decode := func(body []byte) error {
		_, err := decodeRetirementRecords(body, "/opt/f.forwarding-retirement-00000000-journal", "/opt/f/forwarding-retirement-00000000", hashes[0], hashes[1], hashes[2], hashes[3], 988)
		return err
	}
	if err := decode(raw); err != nil {
		t.Fatal("independent complete transcript grammar refused", err)
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 18 {
		t.Fatal("independent transcript size differs")
	}
	for i := range lines {
		missing := append(bytes.Join(append(append([][]byte(nil), lines[:i]...), lines[i+1:]...), []byte{'\n'}), '\n')
		if decode(missing) == nil {
			t.Fatal("missing original phase admitted", i)
		}
		changed := append([][]byte(nil), lines...)
		changed[i] = bytes.Replace(lines[i], []byte("ardents-forwarding-retirement-journal-v1"), []byte("ardents-source-retirement-record-v1"), 1)
		if decode(append(bytes.Join(changed, []byte{'\n'}), '\n')) == nil {
			t.Fatal("foreign purpose schema admitted", i)
		}
		if i > 0 {
			changed = append([][]byte(nil), lines...)
			changed[i-1], changed[i] = changed[i], changed[i-1]
			if decode(append(bytes.Join(changed, []byte{'\n'}), '\n')) == nil {
				t.Fatal("reordered physical phase admitted", i)
			}
		}
	}
	for _, changed := range [][]byte{bytes.Replace(raw, []byte(`"inode":10`), []byte(`"inode":0`), 1), bytes.Replace(raw, []byte(`"gid":988`), []byte(`"gid":989`), 1), bytes.Replace(raw, []byte(`"uid":0`), []byte(`"uid":988`), 1), raw[:len(raw)-1], append(bytes.Clone(raw), '\n')} {
		if decode(changed) == nil {
			t.Fatal("foreign native identity or framing admitted")
		}
	}
}

func TestForwardingRetirementChallengeRequiresExactFreshNonzeroBytes(t *testing.T) {
	text := strings.Repeat("ab", 32)
	value, err := retirementChallenge(text)
	if err != nil || value[0] != 0xab {
		t.Fatal("exact byte grammar refused", err)
	}
	for _, other := range []string{"", strings.Repeat("0", 64), strings.Repeat("AB", 32), text[:62], text + "00"} {
		if _, err := retirementChallenge(other); err == nil {
			t.Fatal("foreign challenge grammar admitted", other)
		}
	}
}

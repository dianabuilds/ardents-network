package forwarding

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
)

// decodeInitialRecords consumes the entire distinct initial grammar. Historical
// account records describe the original helper, not immutable host databases.
// Returned native numbers are untrusted observations until independently matched;
// neither complete bytes nor this parser grants Release or startup authority.
func decodeInitialRecords(raw []byte, b binding, hashes map[string]string) (map[string]initialRecord, error) {
	if len(raw) == 0 || len(raw) > 4<<20 || raw[len(raw)-1] != '\n' || len(hashes) != 17 {
		return nil, ErrBinding
	}
	bound, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	bound = append(bound, '\n')
	if _, err := decodeBinding(bound); err != nil {
		return nil, err
	}
	boundSHA := digest(bound)
	base := filepath.Join(b.Request.InstallationRoot, "generations")
	sealed := filepath.Join(base, b.Generation)
	for path, wanted := range map[string]string{
		b.Request.InstallationRoot + ".forwarding-initial-pending":      digest([]byte("ardents-forwarding-initial-pending-v1\n" + boundSHA + "\n")),
		filepath.Join(b.Request.InstallationRoot, "binding.json"):       boundSHA,
		"/etc/systemd/system/ardents-forwarding.service":                b.UnitSHA256,
		filepath.Join(b.Request.CredentialDirectory, "certificate.pem"): b.CertificateSHA256,
		filepath.Join(sealed, "binding.json"):                           boundSHA,
		filepath.Join(sealed, programName):                              b.ProgramSHA256,
		filepath.Join(sealed, "ardents-forwarding.service"):             b.TemplateSHA256,
		filepath.Join(sealed, "infrastructure-forwarding.json"):         b.Generation,
		filepath.Join(sealed, "infrastructure-enrollment.json"):         b.InventorySHA256,
		filepath.Join(sealed, "targets.json"):                           b.TargetsSHA256,
		filepath.Join(sealed, "forwarding-request.json"):                b.RequestSHA256,
		filepath.Join(sealed, "forwarding-plan.json"):                   b.PlanSHA256,
		filepath.Join(sealed, "rendered-forwarding.service"):            b.UnitSHA256,
	} {
		if hashes[path] != wanted {
			return nil, ErrBinding
		}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 119 {
		return nil, ErrBinding
	}
	records := make([]initialRecord, len(lines))
	for i, line := range lines {
		if len(line)+1 > 32<<10 || json.Unmarshal(line, &records[i]) != nil {
			return nil, ErrBinding
		}
		canonical, err := json.Marshal(records[i])
		if err != nil || !bytes.Equal(canonical, line) || records[i].Sequence != i {
			return nil, ErrBinding
		}
	}
	position := 0
	next := func(wanted initialRecord, native bool) (initialRecord, error) {
		if position >= len(records) {
			return initialRecord{}, ErrBinding
		}
		actual := records[position]
		wanted.Sequence = position
		if native {
			if actual.Device == 0 || actual.Inode == 0 {
				return initialRecord{}, ErrBinding
			}
			wanted.Device, wanted.Inode = actual.Device, actual.Inode
		}
		if actual != wanted {
			return initialRecord{}, ErrBinding
		}
		position++
		return actual, nil
	}
	journal := b.Request.InstallationRoot + ".forwarding-initial-journal"
	first, err := next(initialRecord{Phase: "forwarding-initial-v1", Path: journal, Mode: 0600}, true)
	if err != nil {
		return nil, err
	}
	owned := map[string]initialRecord{journal: first}
	write := func(path string, mode, uid, gid uint32) error {
		sha := hashes[path]
		if !initialSHA(sha) {
			return ErrBinding
		}
		if _, err := next(initialRecord{Phase: "file-intent", Path: path, SHA256: sha, Mode: mode, UID: uid, GID: gid}, false); err != nil {
			return err
		}
		born, err := next(initialRecord{Phase: "file-born", Path: path, Mode: 0600}, true)
		if err != nil {
			return err
		}
		born.Phase, born.SHA256, born.Mode, born.UID, born.GID = "file-ready", sha, mode, uid, gid
		ready, err := next(born, false)
		if err == nil {
			owned[path] = ready
		}
		return err
	}
	if err := write(b.Request.InstallationRoot+".forwarding-initial-pending", 0600, 0, 0); err != nil {
		return nil, err
	}
	history, err := next(initialRecord{Phase: "admitted", Path: b.History, SHA256: boundSHA, Mode: 0700}, true)
	if err != nil {
		return nil, err
	}
	owned[b.History] = history
	protected := func(phase, path string, optional bool) (initialRecord, error) {
		if position >= len(records) {
			return initialRecord{}, ErrBinding
		}
		r := records[position]
		if optional && r.Device == 0 && r.Inode == 0 {
			return next(initialRecord{Phase: phase, Path: path}, false)
		}
		if r.UID != 0 || r.Mode > 0777 || r.Mode&0022 != 0 || !initialSHA(r.SHA256) {
			return initialRecord{}, ErrBinding
		}
		return next(initialRecord{Phase: phase, Path: path, SHA256: r.SHA256, Mode: r.Mode, GID: r.GID}, true)
	}
	preimages := map[string]initialRecord{}
	for _, path := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"} {
		if preimages[path], err = protected("account-preimage", path, false); err != nil {
			return nil, err
		}
	}
	auxiliary := map[string]initialRecord{}
	for _, path := range nodeAccountAuxiliaryPaths() {
		before, err := protected("account-auxiliary-preimage", path, true)
		if err != nil || before.Inode != 0 && path != "/etc/.pwd.lock" && !strings.HasSuffix(path, "-") {
			return nil, ErrBinding
		}
		auxiliary[path] = before
	}
	if _, err := next(initialRecord{Phase: "account-intent", Path: "ardents-node", UID: b.UID, GID: b.GID}, false); err != nil {
		return nil, err
	}
	for _, path := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"} {
		if _, err := protected("account-born", path, false); err != nil {
			return nil, err
		}
	}
	for _, path := range nodeAccountAuxiliaryPaths() {
		if path == "/etc/.pwd.lock" || strings.HasSuffix(path, "-") {
			after, err := protected("account-auxiliary-result", path, false)
			if err != nil {
				return nil, err
			}
			if path == "/etc/.pwd.lock" {
				before := auxiliary[path]
				if after.Mode != 0600 || after.GID != 0 || after.SHA256 != digest(nil) || before.Inode != 0 && (before.Device != after.Device || before.Inode != after.Inode) {
					return nil, ErrBinding
				}
			} else if after.SHA256 != preimages[strings.TrimSuffix(path, "-")].SHA256 {
				return nil, ErrBinding
			}
		} else if _, err := next(initialRecord{Phase: "account-auxiliary-cleared", Path: path}, false); err != nil {
			return nil, err
		}
	}
	if _, err := next(initialRecord{Phase: "account-ready", Path: "ardents-node", UID: b.UID, GID: b.GID}, false); err != nil {
		return nil, err
	}
	for _, d := range []initialRecord{
		{Path: b.Request.InstallationRoot, Mode: 0755}, {Path: base, Mode: 0755}, {Path: sealed, Mode: 0755},
		{Path: b.Request.StateRoot, Mode: 0700, UID: b.UID, GID: b.GID},
		{Path: b.Request.ParticipationRoot, Mode: 0700, UID: b.UID, GID: b.GID},
		{Path: b.Request.SpendRoot, Mode: 0700, UID: b.UID, GID: b.GID},
		{Path: b.Request.HostingRoot, Mode: 0700, UID: b.UID, GID: b.GID},
		{Path: b.Request.CredentialDirectory, Mode: 0755},
	} {
		d.Phase = "directory-intent"
		if _, err := next(d, false); err != nil {
			return nil, err
		}
		born, err := next(initialRecord{Phase: "directory-born", Path: d.Path, Mode: 0700}, true)
		if err != nil {
			return nil, err
		}
		d.Phase, d.Device, d.Inode = "directory-ready", born.Device, born.Inode
		if d, err = next(d, false); err != nil {
			return nil, err
		}
		owned[d.Path] = d
	}
	if err := write(filepath.Join(b.Request.CredentialDirectory, "certificate.pem"), 0444, 0, 0); err != nil {
		return nil, err
	}
	if err := write(filepath.Join(b.Request.CredentialDirectory, "private-key.pem"), 0640, 0, b.GID); err != nil {
		return nil, err
	}
	names := []string{programName, "ardents-forwarding.service", "infrastructure-forwarding.json", "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json", "forwarding-request.json", "forwarding-plan.json", "rendered-forwarding.service", "binding.json"}
	slices.Sort(names)
	for _, name := range names {
		mode := uint32(0444)
		if name == programName {
			mode = 0555
		}
		if err := write(filepath.Join(sealed, name), mode, 0, 0); err != nil {
			return nil, err
		}
	}
	d := owned[sealed]
	d.Mode, d.SHA256 = 0555, b.Generation
	for _, phase := range []string{"seal-intent", "sealed"} {
		d.Phase = phase
		if d, err = next(d, false); err != nil {
			return nil, err
		}
	}
	owned[sealed] = d
	selection := filepath.Join(b.Request.InstallationRoot, "binding.json")
	if err := write(selection, 0644, 0, 0); err != nil {
		return nil, err
	}
	if _, err := next(initialRecord{Phase: "selected", Path: selection, SHA256: boundSHA, Mode: 0644}, false); err != nil {
		return nil, err
	}
	unit := "/etc/systemd/system/ardents-forwarding.service"
	if err := write(unit, 0644, 0, 0); err != nil {
		return nil, err
	}
	for _, r := range []initialRecord{
		{Phase: "reload-intent", Path: unit, SHA256: b.UnitSHA256, Mode: 0644},
		{Phase: "stopped-verified", Path: selection, SHA256: boundSHA, Mode: 0644},
		{Phase: "completed", Path: journal, SHA256: boundSHA, Mode: 0600},
	} {
		if _, err := next(r, false); err != nil {
			return nil, err
		}
	}
	if position != len(records) || len(owned) != 27 {
		return nil, ErrBinding
	}
	identities := map[[2]uint64]bool{}
	for _, record := range owned {
		key := [2]uint64{record.Device, record.Inode}
		if identities[key] {
			return nil, ErrBinding
		}
		identities[key] = true
	}
	return owned, nil
}

func initialSHA(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

//go:build network_signing_native

package descriptor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
)

// This is an actual same-program native process/channel/journal test. Its
// detached binding and explicit non-native context supply no signed Release,
// installed transaction, safe continuation, manager or domain-join authority.
func TestDescriptorTransitionScopeNativeFromBirth(t *testing.T) {
	if path := os.Getenv("ARDENTS_DESCRIPTOR_PHYSICAL_TRANSITION_BINDING"); path != "" {
		nativePhysicalTransitionChild(t, path)
		return
	}
	marker, err := os.ReadFile("/etc/ardents-descriptor-preparation-native-stand")
	if err != nil || string(marker) != "ardents-descriptor-preparation-native-v1\n" || os.Getuid() != 0 || os.Getgid() != 0 {
		t.Fatal("invalid environment: explicit disposable root preparation stand required", err)
	}
	for _, mode := range []string{"ordinary", "cut", "cancel-frozen", "cancel-outstanding", "proof-close", "reader-join", "reader-close"} {
		t.Run(mode, func(t *testing.T) { nativePhysicalTransitionParent(t, mode) })
	}
}

func nativePhysicalTransitionChild(t *testing.T, path string) {
	t.Helper()
	action := "replace"
	c, selected, err := OpenTransitionCoordinator(action)
	if err != nil || !selected || c == nil {
		t.Fatal("actual inherited channel", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	}()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := decodeSelection(raw)
	if err != nil {
		t.Fatal(err)
	}
	root := selection.binding.Request.InstallationRoot
	journal, effect := replacementJournal(root, selection.binding.Generation), root+".physical-effect"
	// Deliberately not a valid native-context schema: no recovery may accept it.
	nativeContext := []byte("{\"schema\":\"physical-test-only\"}\n")
	if err := bindTransition(c.Context(), "replacement", journal, selection.binding.Generation, raw, nativeContext, "physical-test-only"); err != nil {
		t.Fatal(err)
	}
	writer, err := openDescriptorBirths(c.Context(), journal, map[string]bool{root: true, effect: true}, "descriptor-replacement-v1")
	defer func() {
		if err := writer.close(); err != nil {
			t.Error(err)
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.record("native-context", root, nil, digest(nativeContext), 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := writer.write(effect, []byte("actual effect after acknowledged boundary\n"), 0600, 0, 0, writer.check); err != nil {
		t.Fatal(err)
	}
	if err := writer.close(); err != nil {
		t.Fatal(err)
	}
}

func nativePhysicalTransitionParent(t *testing.T, mode string) {
	t.Helper()
	parent, err := os.MkdirTemp(os.TempDir(), "ftc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	b := nativeDescriptorPhysicalBinding(t)
	b.Request.InstallationRoot = filepath.Join(parent, "i")
	request, _ := json.Marshal(b.Request)
	b.RequestSHA256 = digest(append(request, '\n'))
	unit, err := renderUnit([]byte(deployment.DescriptorTemplate), b.Request, b.Generation)
	if err != nil {
		t.Fatal(err)
	}
	b.UnitSHA256 = digest(unit)
	raw, _ := json.Marshal(b)
	raw = append(raw, '\n')
	if _, err := decodeSelection(raw); err != nil {
		t.Fatal("detached physical binding", err)
	}
	operation := "replacement"
	path := filepath.Join(parent, "binding.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	program, err := os.Open("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	programClosed := false
	defer func() {
		if !programClosed {
			if err := program.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	channel, endpoint, err := transitionSocketPair()
	if err != nil {
		t.Fatal(err)
	}
	channelClosed := false
	defer func() {
		if !channelClosed {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	nonce := strings.Repeat("a", 64)
	environment := append(os.Environ(), "ARDENTS_DESCRIPTOR_TRANSITION_CHILD=coordinator-v1", "ARDENTS_DESCRIPTOR_TRANSITION_FD=4", "ARDENTS_DESCRIPTOR_TRANSITION_NONCE="+nonce, "ARDENTS_DESCRIPTOR_PHYSICAL_TRANSITION_BINDING="+path)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	scope, err := forkTransition(program, endpoint, []string{os.Args[0], "-test.run=^TestDescriptorTransitionScopeNativeFromBirth$", "-test.timeout=30s"}, environment)
	endpointErr := endpoint.Close()
	if err != nil {
		t.Fatal(errors.Join(err, endpointErr))
	}
	defer func() {
		if err := scope.join(); err != nil {
			t.Error(err)
		}
	}()
	if endpointErr != nil {
		t.Fatal(endpointErr)
	}
	origin, err := captureTransitionOrigin(program, scope)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	stopAfter := uint32(0)
	if mode == "cut" || mode == "proof-close" || mode == "reader-join" || mode == "reader-close" {
		stopAfter = 2
	}
	if mode == "cancel-frozen" {
		boundaries := 0
		scope.frozen = func() {
			boundaries++
			if boundaries == 2 {
				cancel()
			}
		}
	}
	if mode == "cancel-outstanding" {
		scope.acknowledged = cancel
	}
	cut, err := controlTransition(ctx, scope, channel, nonce, operation, time.Now().Add(20*time.Second), stopAfter)
	if mode == "cancel-frozen" || mode == "cancel-outstanding" {
		if !errors.Is(err, context.Canceled) {
			t.Fatal("original cancellation disappeared", err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	effect := b.Request.InstallationRoot + ".physical-effect"
	if mode == "cancel-outstanding" {
		if cut != nil || !scope.status.Signaled() || scope.reaped != 1 {
			t.Fatal("outstanding ACK became a frozen cut", scope.status, scope.reaped)
		}
		if _, err := os.Lstat(effect); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("cancelled next effect occurred", err)
		}
		return
	}
	if mode == "ordinary" {
		if cut != nil || !scope.status.Exited() || scope.status.ExitStatus() != 0 || scope.reaped != 1 {
			t.Fatal("ordinary actor did not join", scope.status, scope.reaped)
		}
		actual, err := os.ReadFile(effect)
		if err != nil || string(actual) != "actual effect after acknowledged boundary\n" {
			t.Fatal("acknowledged real effect unavailable", err)
		}
		return
	}
	if cut == nil || !cut.exclusive || cut.message.Sequence != 2 || scope.reaped != 1 {
		t.Fatal("actual frozen boundary lost", cut, scope.status, scope.reaped)
	}
	defer func() {
		if err := cut.close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := os.Lstat(effect); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("denied next effect occurred", err)
	}
	if err := errors.Join(program.Close(), channel.Close()); err != nil {
		t.Fatal(err)
	}
	programClosed, channelClosed = true, true
	completion, finish := context.WithTimeout(context.Background(), 3*time.Second)
	defer finish()
	if mode == "proof-close" {
		nativePhysicalCutClose(t, completion, cut, scope, origin)
		return
	}
	if err := publishTransitionCut(completion, cut, scope, origin); err != nil {
		t.Fatal("physical cut publication", err)
	}
	controlPath, resultPath, pending := transitionCutPaths(cut.message.Journal)
	if _, err := os.Lstat(pending); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("proof guard retained", err)
	}
	control, controlErr := os.ReadFile(controlPath)
	result, resultErr := os.ReadFile(resultPath)
	controlInfo, controlStat := os.Lstat(controlPath)
	resultInfo, resultStat := os.Lstat(resultPath)
	if err := errors.Join(controlErr, resultErr, controlStat, resultStat); err != nil {
		t.Fatal(err)
	}
	proof, err := decodeTransitionCut(control, result, cut.journal.body, controlInfo, resultInfo, cut.journal.info)
	if err != nil || proof.Origin != origin || proof.Acknowledged != 1 || !bytes.Equal(proof.Boundary.Binding, raw) {
		t.Fatal("independent physical proof readback", err)
	}
	independent, err := openRecoveryOriginal(completion, cut.message.Journal, nil)
	if err != nil {
		t.Fatal("fresh original historical custody", err)
	}
	if mode == "reader-join" || mode == "reader-close" {
		nativePhysicalOriginalJoin(t, independent, cut.message.Journal, mode == "reader-close")
		return
	}
	lease, err := os.OpenFile(cut.message.Journal, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(errors.Join(err, independent.close()))
	}
	held := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	checkErr, closed := independent.check(), independent.close()
	released := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	leaseClose := lease.Close()
	if !errors.Is(held, syscall.EWOULDBLOCK) || checkErr != nil || closed != nil || released != nil || leaseClose != nil {
		t.Fatal("fresh original lease did not span independent proof users", held, checkErr, closed, released, leaseClose)
	}
}

func nativePhysicalCutClose(t *testing.T, ctx context.Context, cut *transitionPrefix, scope *transitionScope, origin transitionOrigin) {
	t.Helper()
	controlPath, resultPath, pending := transitionCutPaths(cut.message.Journal)
	held, release := make(chan struct{}), make(chan struct{})
	late := errors.New("physical proof Close refusal")
	cut.beforeProofClose = func(file *os.File) error {
		if file.Name() != controlPath {
			return nil
		}
		close(held)
		<-release
		return late
	}
	done := make(chan error, 1)
	go func() { done <- publishTransitionCut(ctx, cut, scope, origin) }()
	select {
	case <-held:
	case err := <-done:
		t.Fatal("required original proof Close was not held", err)
	}
	// Accumulate checks before releasing the real Close, so a failing assertion
	// cannot strand that writer or the retained independent original lease.
	_, guardErr := os.Lstat(pending)
	result, resultErr := os.ReadFile(resultPath)
	file, openErr := os.OpenFile(cut.message.Journal, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	var leaseErr, closeErr error
	if openErr == nil {
		leaseErr = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		closeErr = file.Close()
	}
	close(release)
	publication := <-done
	if guardErr != nil || resultErr != nil || len(result) == 0 || openErr != nil || closeErr != nil || !errors.Is(leaseErr, syscall.EWOULDBLOCK) || !errors.Is(publication, late) {
		t.Fatal("required Close lost guard, lease or independent failure", guardErr, resultErr, openErr, leaseErr, closeErr, publication)
	}
	if _, err := os.Lstat(pending); err != nil {
		t.Fatal("complete-looking proof accepted after late Close refusal", err)
	}
	independent, err := openRecoveryOriginal(ctx, cut.message.Journal, nil)
	if err == nil || independent != nil || !errors.Is(err, ErrBinding) {
		t.Fatal("guarded complete-looking proof supplied fresh historical custody", err)
	}
}

// Detached declarations exercise only native process/journal/channel custody;
// they are never Release, installed-history, State or domain-join authority.
func nativeDescriptorPhysicalBinding(t *testing.T) binding {
	t.Helper()
	r, err := DecodeRequest([]byte(independentRequest))
	if err != nil {
		t.Fatal(err)
	}
	sha := func(c string) string { return strings.Repeat(c, 64) }
	unit, err := renderUnit([]byte(deployment.DescriptorTemplate), r, sha("1"))
	if err != nil {
		t.Fatal(err)
	}
	return binding{Schema: "ardents-descriptor-binding-v1", Invocation: "descriptor", Generation: sha("1"), ProgramTarget: "ardents/linux-amd64/node", GenerationTarget: "ardents/linux-amd64/infrastructure/descriptor", ProgramSHA256: sha("2"), TemplateSHA256: digest([]byte(deployment.DescriptorTemplate)), InventorySHA256: sha("3"), TargetsSHA256: sha("4"), RequestSHA256: digest([]byte(independentRequest)), PlanSHA256: sha("5"), UnitSHA256: digest(unit), CertificateSHA256: sha("6"), Account: "ardents-node", UID: 999, GID: 999, History: "/var/lib/release-history", Request: r}
}

// Close seals admission synchronously, joins the original reader and retains
// every late physical Close error before releasing the original journal lease.
func nativePhysicalOriginalJoin(t *testing.T, original *recoveryOriginal, journal string, failClose bool) {
	t.Helper()
	lease, err := os.OpenFile(journal, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	}()
	late := errors.New("independent original reader late Close failure")
	held, release := make(chan struct{}), make(chan struct{})
	first := true
	original.observe = func(phase string, file *os.File) error {
		if phase == "read" && first {
			first = false
			close(held)
			<-release
		}
		if failClose && phase == "before-close" {
			return late
		}
		return nil
	}
	checked := make(chan error, 1)
	go func() { checked <- original.check() }()
	<-held
	closed := make(chan error, 1)
	go func() { closed <- original.close() }()
	for {
		original.mu.Lock()
		sealed := original.closed
		original.mu.Unlock()
		if sealed {
			break
		}
		runtime.Gosched()
	}
	// The actual original reader retains the flock until physical closure.
	heldErr := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	select {
	case err := <-closed:
		close(release)
		<-checked
		t.Fatal("Close returned before original reader joined", err)
	default:
	}
	close(release)
	checkErr, closeErr := <-checked, <-closed
	if !errors.Is(heldErr, syscall.EWOULDBLOCK) {
		t.Fatal("original reader lost its lease", heldErr)
	}
	if !errors.Is(checkErr, ErrInput) {
		t.Fatal("original reader unexpected failure", checkErr)
	}
	if failClose {
		if !errors.Is(closeErr, late) || !errors.Is(original.close(), late) {
			t.Fatal("late Close error disappeared", closeErr)
		}
	} else if !errors.Is(closeErr, ErrInput) {
		t.Fatal("original sealed-reader refusal disappeared", closeErr)
	}
	if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("joined original lease not returned", err)
	}
}

//go:build linux && network_preparation_native

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func preparationPrivateParent(t *testing.T) string {
	t.Helper()
	p := t.TempDir()
	if err := os.Chmod(p, 0700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNetworkPreparationNativeCustody(t *testing.T) {
	message := bytes.Repeat([]byte("public canonical bytes"), 128)
	for _, fault := range []string{"healthy", "existing directory", "ancestor symlink", "file symlink", "file replacement", "hardlink", "directory replacement", "parent replacement", "extra file", "late extra file", "corrupt readback", "physical write close", "physical directory sync"} {
		t.Run(fault, func(t *testing.T) {
			parent := preparationPrivateParent(t)
			root := filepath.Join(parent, "prepared")
			name := "epoch.unsigned"
			if fault == "existing directory" {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "ancestor symlink" {
				alias := filepath.Join(parent, "alias")
				if err := os.Symlink(parent, alias); err != nil {
					t.Fatal(err)
				}
				root = filepath.Join(alias, "prepared")
			}
			observe := func(step string, f *os.File) {
				var err error
				if step == "file-created" {
					switch fault {
					case "file symlink":
						err = os.Rename(filepath.Join(root, name), filepath.Join(parent, "original"))
						if err == nil {
							err = os.Symlink(filepath.Join(parent, "original"), filepath.Join(root, name))
						}
					case "file replacement":
						err = os.Rename(filepath.Join(root, name), filepath.Join(parent, "original"))
						if err == nil {
							err = os.WriteFile(filepath.Join(root, name), message, 0600)
						}
					case "hardlink":
						err = os.Link(filepath.Join(root, name), filepath.Join(parent, "alias"))
					case "directory replacement":
						err = os.Rename(root, root+"-original")
						if err == nil {
							err = os.Mkdir(root, 0700)
						}
					case "parent replacement":
						err = os.Rename(parent, parent+"-original")
						if err == nil {
							err = os.Mkdir(parent, 0700)
						}
						t.Cleanup(func() {
							if err := os.Remove(parent); err != nil {
								t.Error(err)
							}
							if err := os.Rename(parent+"-original", parent); err != nil {
								t.Error(err)
							}
						})
					case "extra file":
						err = os.WriteFile(filepath.Join(root, "foreign"), []byte("keep"), 0600)
					case "physical write close":
						err = f.Close()
					}
				}
				if step == "closed" && fault == "corrupt readback" {
					err = os.WriteFile(filepath.Join(root, name), []byte("corrupt"), 0600)
				}
				if step == "before-directory-sync" && fault == "physical directory sync" {
					err = f.Close()
				}
				if step == "before-directory-sync" && fault == "late extra file" {
					err = os.WriteFile(filepath.Join(root, "foreign"), []byte("keep"), 0600)
				}
				if err != nil {
					t.Fatal(step, err)
				}
			}
			err := persistNetworkPreparation(t.Context(), root, name, message, observe)
			if fault == "healthy" {
				if err != nil {
					t.Fatal(err)
				}
				saved, e := os.ReadFile(filepath.Join(root, name))
				if e != nil || !bytes.Equal(saved, message) {
					t.Fatal("exact durable bytes", e)
				}
			} else if err == nil {
				t.Fatal("native mutation falsely succeeded")
			}
			if fault == "physical write close" && (!errors.Is(err, os.ErrClosed) || strings.Count(err.Error(), "file already closed") < 3) {
				t.Fatal("independent write/sync/close failures lost", err)
			}
		})
	}
}

func TestNetworkPreparationCancellationRetainsIncompletePrefix(t *testing.T) {
	for _, stage := range []string{"before", "directory-created", "file-created", "written", "closed", "readback", "before-directory-sync"} {
		t.Run(stage, func(t *testing.T) {
			parent := preparationPrivateParent(t)
			root := filepath.Join(parent, "prepared")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if stage == "before" {
				cancel()
			}
			err := persistNetworkPreparation(ctx, root, "profile.unsigned", []byte("public unsigned"), func(step string, _ *os.File) {
				if step == stage {
					cancel()
				}
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatal("original cancellation lost", err)
			}
			_, stat := os.Stat(root)
			if stage == "before" {
				if !errors.Is(stat, os.ErrNotExist) {
					t.Fatal("pre-cancel effect", stat)
				}
				return
			}
			if stat != nil {
				t.Fatal("uncertain prefix cleaned up", stat)
			}
			if err := writeNetworkPreparation(t.Context(), root, "profile.unsigned", []byte("public unsigned")); err == nil {
				t.Fatal("incomplete prefix adopted")
			}
		})
	}
}

func TestNetworkPreparationInterruptedWriterChild(t *testing.T) {
	if os.Getenv("ARDENTS_PREPARATION_CHILD") != "1" {
		return
	}
	if os.Getenv("ARDENTS_PREPARATION_STAGE") == "file-limit" {
		// Change only this isolated child, never host/global process limits.
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 16, Max: 16}); err != nil {
			t.Fatal(err)
		}
		root := os.Getenv("ARDENTS_PREPARATION_ROOT")
		err := writeNetworkPreparation(t.Context(), root, "epoch.unsigned", bytes.Repeat([]byte{1}, 4096))
		if !errors.Is(err, syscall.EFBIG) {
			t.Fatal("actual partial write cause lost", err)
		}
		saved, e := os.ReadFile(filepath.Join(root, "epoch.unsigned"))
		if e != nil || len(saved) != 16 {
			t.Fatal("actual partial prefix", len(saved), e)
		}
		return
	}
	err := persistNetworkPreparation(context.Background(), os.Getenv("ARDENTS_PREPARATION_ROOT"), "epoch.unsigned", []byte("public unsigned"), func(step string, _ *os.File) {
		if step == os.Getenv("ARDENTS_PREPARATION_STAGE") {
			_, _ = os.Stdout.Write([]byte("held\n"))
			time.Sleep(time.Minute)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNetworkPreparationActualPartialWrite(t *testing.T) {
	parent := preparationPrivateParent(t)
	root := filepath.Join(parent, "prepared")
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestNetworkPreparationInterruptedWriterChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), "ARDENTS_PREPARATION_CHILD=1", "ARDENTS_PREPARATION_ROOT="+root, "ARDENTS_PREPARATION_STAGE=file-limit")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal("finite partial-write child", err, string(out))
	}
	if err := writeNetworkPreparation(t.Context(), root, "epoch.unsigned", bytes.Repeat([]byte{1}, 4096)); err == nil {
		t.Fatal("partial native output adopted")
	}
}

func TestNetworkPreparationUnsupportedMountRefusesBeforeBirth(t *testing.T) {
	parent, err := os.MkdirTemp("/dev/shm", "ardents-public-preparation-")
	if err != nil {
		t.Fatal("tmpfs refusal prerequisite unavailable", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	root := filepath.Join(parent, "prepared")
	if err := writeNetworkPreparation(t.Context(), root, "epoch.unsigned", []byte("public")); err == nil {
		t.Fatal("unsupported tmpfs durability admitted")
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unsupported mount caused effects", err)
	}
}

func TestNetworkPreparationActualProcessInterruption(t *testing.T) {
	for _, stage := range []string{"directory-created", "file-created", "written", "closed", "before-directory-sync"} {
		t.Run(stage, func(t *testing.T) {
			parent := preparationPrivateParent(t)
			root := filepath.Join(parent, "prepared")
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNetworkPreparationInterruptedWriterChild$", "-test.count=1")
			cmd.Env = append(os.Environ(), "ARDENTS_PREPARATION_CHILD=1", "ARDENTS_PREPARATION_ROOT="+root, "ARDENTS_PREPARATION_STAGE="+stage)
			pipe, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			mark := make([]byte, 5)
			_, readErr := io.ReadFull(pipe, mark)
			killErr := cmd.Process.Kill()
			waitErr := cmd.Wait()
			if readErr != nil || string(mark) != "held\n" || killErr != nil || waitErr == nil {
				t.Fatal("original process cut/join unavailable", readErr, killErr, waitErr, stderr.String())
			}
			if _, err := os.Stat(root); err != nil {
				t.Fatal("prefix missing", err)
			}
			if err := writeNetworkPreparation(t.Context(), root, "epoch.unsigned", []byte("public unsigned")); err == nil {
				t.Fatal("process prefix adopted as success")
			}
		})
	}
}

func TestNetworkPreparationNativeInputAndReceiptRefusals(t *testing.T) {
	parent := preparationPrivateParent(t)
	input := filepath.Join(parent, "plan")
	key := bytes.Repeat([]byte{7}, 32)
	raw := preparationJSON(t, preparationRecordPlan(key))
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"symlink", "hardlink", "oversize", "malformed", "output failure", "short receipt"} {
		t.Run(fault, func(t *testing.T) {
			path := input
			out := io.Writer(io.Discard)
			if fault == "symlink" {
				path = filepath.Join(parent, "alias")
				if err := os.Symlink(input, path); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "hardlink" {
				path = filepath.Join(parent, "hardlink")
				if err := os.Link(input, path); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Remove(path); err != nil {
						t.Error(err)
					}
				})
			}
			if fault == "oversize" {
				path = filepath.Join(parent, "large")
				if err := os.WriteFile(path, bytes.Repeat([]byte(" "), maximumNetworkPreparationPlan+1), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "malformed" {
				path = filepath.Join(parent, "bad")
				if err := os.WriteFile(path, append(bytes.Clone(raw), raw...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "output failure" {
				out = preparationFailedWriter{}
			}
			if fault == "short receipt" {
				out = preparationShortWriter{}
			}
			root := filepath.Join(parent, "output-"+fault)
			if code := runNetworkPreparation(t.Context(), []string{"prepare-initial-node-record", path, root}, out); code == 0 {
				t.Fatal("false successful receipt")
			}
			if fault != "output failure" && fault != "short receipt" {
				if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("invalid plan caused output effects", err)
				}
			}
		})
	}
}

type preparationFailedWriter struct{}

func (preparationFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type preparationShortWriter struct{}

func (preparationShortWriter) Write([]byte) (int, error) { return 0, nil }

func TestNetworkPreparationReceiptInterruptsActualBlockedPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	})
	fd := int(w.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	for {
		_, err := syscall.Write(fd, bytes.Repeat([]byte{1}, 4096))
		if errors.Is(err, syscall.EAGAIN) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- writeNetworkPreparationReceipt(ctx, w, []byte("public receipt\n")) }()
	select {
	case err := <-done:
		t.Fatal("full pipe completed before cancellation", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked receipt did not join")
	}
}

package installation

import (
	"bytes"
	"errors"
	"testing"
)

func TestFixedResourceImagesRetainClosedManifest(t *testing.T) {
	files := map[string][]byte{
		"ardents-text-linux-amd64":        []byte("x"),
		"ardents-text-reader@.service":    []byte("x"),
		"ardents-text-publisher@.service": []byte("x"),
		"ardents-text-reader.socket":      []byte("x"),
		"ardents-text-publisher.socket":   []byte("x"),
		"50-ardents-text.rules":           []byte("x"),
		"ardents-text.conf":               []byte("confinement"),
		"endpoint-unit.service":           []byte("rendered"),
		"ardents-endpoint.service":        []byte("unrendered template"),
		"foreign":                         []byte("not a fixed resource"),
	}
	images, err := fixedResourceImages(files)
	if err != nil || len(images) != 9 {
		t.Fatalf("closed images: count=%d err=%v", len(images), err)
	}
	// Independently fixed SHA-256 of "x", closed path roster and JSON order.
	wanted := `{"schema":"ardents-text-worker-artifact-v1","files":{"/etc/systemd/system/ardents-text-publisher.socket":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881","/etc/systemd/system/ardents-text-publisher@.service":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881","/etc/systemd/system/ardents-text-reader.socket":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881","/etc/systemd/system/ardents-text-reader@.service":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881","/usr/lib/ardents/text-worker-root/ardents-text":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881","/usr/share/polkit-1/rules.d/50-ardents-text.rules":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881"}}` + "\n"
	if !bytes.Equal(images["/etc/ardents/text-worker-artifact.json"], []byte(wanted)) {
		t.Fatal("canonical artifact manifest differs from independent bytes")
	}
	if !bytes.Equal(images["/etc/systemd/system/ardents-endpoint.service"], []byte("rendered")) || !bytes.Equal(images["/usr/lib/tmpfiles.d/ardents-text.conf"], []byte("confinement")) {
		t.Fatal("fixed images lost rendered unit or confinement bytes")
	}
	files["endpoint-unit.service"] = []byte("new rendered unit")
	files["ardents-text.conf"] = []byte("new confinement")
	updated, err := fixedResourceImages(files)
	if err != nil || !bytes.Equal(updated["/etc/ardents/text-worker-artifact.json"], []byte(wanted)) {
		t.Fatal("excluded local images changed the closed worker manifest")
	}
	delete(files, "ardents-text-reader.socket")
	if partial, err := fixedResourceImages(files); !errors.Is(err, ErrBinding) || partial != nil {
		t.Fatalf("missing fixed resource: result=%v err=%v", partial, err)
	}
}

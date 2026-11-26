package runtime

import "testing"

func TestDetachedReaderResultCannotExposeBytes(t *testing.T) {
	for _, result := range []*ReaderResult{nil, {}, {body: []byte("private")}} {
		if body, err := result.TakeBytes(); body != nil || err == nil {
			t.Fatal("unjoined or detached result exposed bytes")
		}
	}
	original := &ReaderResult{body: []byte("private")}
	original.self = original
	copy := &ReaderResult{self: original, body: original.body}
	copy.Close()
	if string(original.body) != "private" || original.used {
		t.Fatal("copied result erased its original bytes")
	}
	original.Close()
	if original.body != nil || !original.used {
		t.Fatal("original result did not erase private output")
	}
}

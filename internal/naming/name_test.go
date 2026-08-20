package naming

import (
	"testing"
)

func TestParseValidAndInvalid(t *testing.T) {
	t.Parallel()

	valid := []string{
		"alice",
		"blog.alice",
		"blog-42.example-node",
		"a.b.c.d.e",
	}
	for _, name := range valid {
		if _, err := Parse(name); err != nil {
			t.Errorf("Parse(%q) = error %v", name, err)
		}
	}

	invalid := []string{
		"",
		" Alice",
		"alice.",
		".alice",
		"alice..bob",
		"Alice",
		"alice_foo",
		"aa..",
	}
	for _, name := range invalid {
		if _, err := Parse(name); err == nil {
			t.Errorf("Parse(%q) = nil, want error", name)
		}
	}
}

func TestParseAndFormatServiceLink(t *testing.T) {
	t.Parallel()

	name, err := ParseServiceLink("ardents://blog.example")
	if err != nil {
		t.Fatalf("ParseServiceLink: %v", err)
	}
	if name != "blog.example" {
		t.Fatalf("ParseServiceLink = %q, want %q", name, "blog.example")
	}
	link, err := FormatServiceLink(name)
	if err != nil {
		t.Fatalf("FormatServiceLink: %v", err)
	}
	if link != "ardents://blog.example" {
		t.Fatalf("FormatServiceLink = %q", link)
	}

	for _, raw := range []string{
		"ARDENTS://blog.example",
		"ardents://Blog.Example",
		" ardents://blog.example",
		"ardents://blog.example ",
	} {
		if _, err := ParseServiceLink(raw); err == nil {
			t.Errorf("ParseServiceLink(%q) accepted a non-canonical link", raw)
		}
	}
}

func TestLabelsAndDescendant(t *testing.T) {
	t.Parallel()

	labels, err := labelsOf("blog.example")
	if err != nil {
		t.Fatalf("labelsOf: %v", err)
	}
	if len(labels) != 2 || labels[0] != "blog" || labels[1] != "example" {
		t.Fatalf("labelsOf = %v", labels)
	}
	if !IsDescendant("blog.example", "example") {
		t.Fatalf("expected blog.example descendant of example")
	}
	if IsDescendant("example", "blog.example") {
		t.Fatalf("expected example not descendant of blog.example")
	}
}

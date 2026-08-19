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

func TestParseServiceLinkAndCanonicalize(t *testing.T) {
	t.Parallel()

	name, err := ParseServiceLink("ardents://Blog.Example")
	if err != nil {
		t.Fatalf("ParseServiceLink: %v", err)
	}
	if name != "blog.example" {
		t.Fatalf("ParseServiceLink = %q, want %q", name, "blog.example")
	}

	canonical, err := canonicalize("  Blog.Example ")
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	if canonical != "blog.example" {
		t.Fatalf("canonicalize = %q, want %q", canonical, "blog.example")
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
	if !isDescendant("blog.example", "example") {
		t.Fatalf("expected blog.example descendant of example")
	}
	if isDescendant("example", "blog.example") {
		t.Fatalf("expected example not descendant of blog.example")
	}
}

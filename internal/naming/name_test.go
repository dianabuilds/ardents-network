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

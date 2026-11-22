package main

import "strings"

// nodeRepeatableFlag is the shared bounded repeatable string option grammar for
// the fixed plan producers. Each caller fixes its own maximum and option name;
// the grammar holds no role policy.
type nodeRepeatableFlag struct {
	values  []string
	maximum int
}

func (f *nodeRepeatableFlag) String() string { return strings.Join(f.values, ",") }
func (f *nodeRepeatableFlag) Set(value string) error {
	if len(f.values) >= f.maximum {
		return errSourceControlInput
	}
	f.values = append(f.values, value)
	return nil
}

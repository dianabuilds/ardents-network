package config

import (
	"reflect"
	"sort"
	"strings"
)

func changedPaths(previous, next Document) []string {
	var paths []string
	diffValue(reflect.ValueOf(previous), reflect.ValueOf(next), "", &paths)
	sort.Strings(paths)
	return paths
}

func diffValue(previous, next reflect.Value, path string, paths *[]string) {
	if previous.Type() != next.Type() {
		*paths = append(*paths, path)
		return
	}
	if previous.Kind() != reflect.Struct {
		if !reflect.DeepEqual(previous.Interface(), next.Interface()) {
			*paths = append(*paths, path)
		}
		return
	}
	typeInfo := previous.Type()
	for index := 0; index < previous.NumField(); index++ {
		name := strings.Split(typeInfo.Field(index).Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		child := name
		if path != "" {
			child = path + "." + name
		}
		diffValue(previous.Field(index), next.Field(index), child, paths)
	}
}

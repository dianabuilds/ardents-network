package service

import "bytes"

type topologyMount struct {
	kind, source, target string
	readOnly             bool
}

func topologyMounts(block []byte) ([]topologyMount, bool) {
	lines := bytes.Split(block, []byte{'\n'})
	var mounts []topologyMount
	var current *topologyMount
	for _, line := range lines {
		if leadingSpaces(line) == 6 && bytes.HasPrefix(bytes.TrimSpace(line), []byte("- ")) &&
			!bytes.HasPrefix(bytes.TrimSpace(line), []byte("- type: ")) {
			return nil, false
		}
		trimmed := bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(line), []byte{'-'}))
		switch {
		case bytes.HasPrefix(trimmed, []byte("type: ")):
			mounts = append(mounts, topologyMount{kind: topologyValue(trimmed, "type")})
			current = &mounts[len(mounts)-1]
		case current != nil && bytes.HasPrefix(trimmed, []byte("source: ")):
			current.source = topologyValue(trimmed, "source")
		case current != nil && bytes.HasPrefix(trimmed, []byte("target: ")):
			current.target = topologyValue(trimmed, "target")
		case current != nil && bytes.HasPrefix(trimmed, []byte("read_only: ")):
			current.readOnly = topologyValue(trimmed, "read_only") == "true"
		}
	}
	for _, mount := range mounts {
		if mount.kind == "" || mount.source == "" || mount.target == "" {
			return nil, false
		}
	}
	return mounts, len(mounts) != 0
}

func topologyValue(line []byte, key string) string {
	return string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte(key+":"))))
}

func topologyDirectValue(block []byte, key string, indent int) string {
	prefix := append(bytes.Repeat([]byte{' '}, indent), key+":"...)
	for _, line := range bytes.Split(block, []byte{'\n'}) {
		if bytes.HasPrefix(line, prefix) && (len(line) == len(prefix) || line[len(prefix)] == ' ') {
			return topologyValue(bytes.TrimSpace(line), key)
		}
	}
	return ""
}

func topologyHasDirectProperty(block []byte, key string, indent int) bool {
	prefix := append(bytes.Repeat([]byte{' '}, indent), key+":"...)
	for _, line := range bytes.Split(block, []byte{'\n'}) {
		if bytes.HasPrefix(line, prefix) && (len(line) == len(prefix) || line[len(prefix)] == ' ' || line[len(prefix)] == '[') {
			return true
		}
	}
	return false
}

func topologyPropertyBlock(raw []byte, marker string, indent int) []byte {
	active := false
	var result []byte
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if string(line) == marker {
			active = true
			continue
		}
		if active && len(bytes.TrimSpace(line)) != 0 && leadingSpaces(line) <= indent {
			break
		}
		if active {
			result = append(result, line...)
			result = append(result, '\n')
		}
	}
	return result
}

func leadingSpaces(line []byte) int {
	for index, value := range line {
		if value != ' ' {
			return index
		}
	}
	return len(line)
}

func topologyIndentedNames(block []byte, marker string, indent int) []string {
	active := false
	var names []string
	for _, line := range bytes.Split(block, []byte{'\n'}) {
		if string(line) == marker {
			active = true
			continue
		}
		if active && len(bytes.TrimSpace(line)) != 0 && leadingSpaces(line) < indent {
			break
		}
		if active && leadingSpaces(line) == indent && line[len(line)-1] == ':' {
			names = append(names, string(line[indent:len(line)-1]))
		}
	}
	return names
}

func topologyIndentedBlock(raw []byte, marker, name string, indent int) []byte {
	parent, active := false, false
	want := append(bytes.Repeat([]byte{' '}, indent), name+":"...)
	var result []byte
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if string(line) == marker {
			parent = true
			continue
		}
		if parent && bytes.Equal(line, want) {
			active = true
			continue
		}
		if active && (len(line) <= indent || !bytes.Equal(line[:indent+1], bytes.Repeat([]byte{' '}, indent+1))) {
			break
		}
		if active {
			result = append(result, line...)
			result = append(result, '\n')
		}
	}
	return result
}

func topologyServices(raw []byte) []string { return topologyIndentedNames(raw, "services:", 2) }

func equalTopologyNames(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := map[string]bool{}
	for _, name := range left {
		seen[name] = true
	}
	for _, name := range right {
		if !seen[name] {
			return false
		}
	}
	return true
}

func topologyServiceBlock(raw []byte, name string) []byte {
	return topologyIndentedBlock(raw, "services:", name, 2)
}

package ubuntu

import "strings"

// Ubuntu2404 accepts the existing infrastructure os-release declaration. The
// caller owns the original file, access checks and reobservation around effects.
func Ubuntu2404(raw []byte) bool {
	fields := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, duplicate := fields[key]; duplicate {
			return false
		}
		fields[key] = strings.Trim(value, "\"")
	}
	return fields["ID"] == "ubuntu" && fields["VERSION_ID"] == "24.04"
}

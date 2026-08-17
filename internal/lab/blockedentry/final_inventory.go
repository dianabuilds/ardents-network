package blockedentry

import (
	"errors"
	"os"
	"path"
	"strconv"
	"strings"
)

func validateCommitmentInventory(file string) error {
	raw, err := os.ReadFile(file)
	if err != nil || len(raw) == 0 || len(raw) > maximumEvidenceFile || raw[len(raw)-1] != '\n' {
		return errors.Join(err, errors.New("final secret inventory is unavailable or non-canonical"))
	}
	previous := ""
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		fields := strings.Split(line, " ")
		if len(fields) != 3 || !hexDigest(fields[0], 32) || strings.ToLower(fields[0]) != fields[0] {
			return errors.New("final secret inventory record is invalid")
		}
		size, sizeErr := strconv.ParseUint(fields[1], 10, 63)
		clean := path.Clean(fields[2])
		if sizeErr != nil || size == 0 || clean != fields[2] || path.IsAbs(clean) || clean == "." ||
			strings.HasPrefix(clean, "../") || strings.ContainsAny(clean, "\\\t\r\n") ||
			previous != "" && clean <= previous {
			return errors.New("final secret inventory path, size, or order is invalid")
		}
		previous = clean
	}
	return nil
}

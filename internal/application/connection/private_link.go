package connection

import "errors"

// ReadPrivateTargetLink reads one Target Link from an owner-private regular
// file. It closes the original handle before returning and withholds local
// filesystem details from the caller.
func ReadPrivateTargetLink(path string) (string, error) {
	unavailable := errors.New("private Target Link file is unavailable")
	input, err := openPrivateLinkFile(path)
	if err != nil {
		return "", unavailable
	}
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		if closeErr := input.Close(); closeErr != nil {
			return "", errors.Join(unavailable, errors.New("private Target Link close failed"))
		}
		return "", unavailable
	}
	link, err := ReadTargetLink(input)
	if errors.Join(err, input.Close()) != nil {
		return "", unavailable
	}
	return link, nil
}

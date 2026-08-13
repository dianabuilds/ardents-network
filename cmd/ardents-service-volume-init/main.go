package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) < 2 {
		return errors.New("usage: ardents-service-volume-init <uid>:<gid> <directory> [<directory>]")
	}
	parts := strings.Split(arguments[0], ":")
	if len(parts) != 2 {
		return errors.New("runtime user must be numeric uid:gid")
	}
	uid, uidErr := strconv.Atoi(parts[0])
	gid, gidErr := strconv.Atoi(parts[1])
	if uidErr != nil || gidErr != nil || uid < 1 || gid < 1 {
		return errors.New("runtime user must be a non-root numeric uid:gid")
	}
	for _, path := range arguments[1:] {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("volume initialization target is not an existing directory")
		}
		if err := os.Chown(path, uid, gid); err != nil {
			return err
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return err
		}
	}
	return nil
}

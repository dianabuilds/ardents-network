package enrollment

import (
	"context"
	"errors"
	"io"
	"os"
)

func sameFileState(before, after os.FileInfo) bool {
	return before != nil && after != nil && before.Mode().IsRegular() && after.Mode().IsRegular() &&
		os.SameFile(before, after) && before.Size() == after.Size() &&
		before.ModTime().Equal(after.ModTime()) && before.Mode() == after.Mode()
}

func readBundleFile(ctx context.Context, root *os.Root, name string, limit int64) (data []byte, info os.FileInfo, err error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	before, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return nil, nil, ErrInventory
	}
	if err := verifyOwnedFile(before); err != nil {
		return nil, nil, err
	}
	file, err := openBundleFile(root, name)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		err = errors.Join(err, file.Close())
		if err != nil {
			data, info = nil, nil
		}
	}()
	opened, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !sameFileState(before, opened) {
		return nil, nil, ErrBinding
	}
	if err := verifyOwnedFile(opened); err != nil {
		return nil, nil, err
	}
	// Fixed-size reads check context and actual bytes; old Stat cannot drive
	// an unbounded allocation when a file grows after inspection.
	var block [32 << 10]byte
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		n, readErr := file.Read(block[:])
		if int64(len(data))+int64(n) > limit {
			return nil, nil, ErrInventory
		}
		data = append(data, block[:n]...)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, nil, readErr
		}
	}
	after, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	pathInfo, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if int64(len(data)) != opened.Size() || !sameFileState(opened, after) || !sameFileState(after, pathInfo) {
		return nil, nil, ErrBinding
	}
	if err := verifyOwnedFile(after); err != nil {
		return nil, nil, err
	}
	if err := verifyOwnedFile(pathInfo); err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return data, after, nil
}

func checkInventory(root *os.Root, entries map[string]string) (err error) {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	items, err := directory.ReadDir(maximumFiles + 2)
	if err != nil && err != io.EOF {
		return err
	}
	if len(items) != len(entries)+1 {
		return ErrInventory
	}
	for _, item := range items {
		if item.Name() == "SHA256SUMS" {
			continue
		}
		if _, ok := entries[item.Name()]; !ok {
			return ErrInventory
		}
	}
	return nil
}

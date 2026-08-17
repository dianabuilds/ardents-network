package modulecache

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func publishCanonicalArchive(root, target string) (returnErr error) {
	paths, err := archivePaths(root)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".ardents-gomodcache-*.partial")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	opened := true
	owned := true
	published := false
	defer func() {
		if opened {
			returnErr = errors.Join(returnErr, temporary.Close())
		}
		if owned {
			returnErr = errors.Join(returnErr, os.Remove(temporaryPath))
		}
		if returnErr != nil && published {
			returnErr = errors.Join(returnErr, os.Remove(target))
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if err := writeCanonicalArchive(temporary, root, paths); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	opened = false
	if err := os.Link(temporaryPath, target); err != nil {
		return err
	}
	published = true
	if err := os.Remove(temporaryPath); err != nil {
		return err
	}
	owned = false
	return nil
}

func writeCanonicalArchive(output io.Writer, root string, paths []string) error {
	gzipWriter := gzip.NewWriter(output)
	gzipWriter.Header.ModTime, gzipWriter.Header.OS = time.Unix(0, 0), 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, path := range paths {
		if err := appendArchivePath(tarWriter, root, path); err != nil {
			return errors.Join(err, tarWriter.Close(), gzipWriter.Close())
		}
	}
	return errors.Join(tarWriter.Close(), gzipWriter.Close())
}

func archivePaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			return errors.New("module cache contains an unsupported file type")
		}
		relative, err := filepath.Rel(root, path)
		if err == nil {
			paths = append(paths, relative)
		}
		return err
	})
	sort.Strings(paths)
	return paths, err
}

func appendArchivePath(output *tar.Writer, root, relative string) error {
	path := filepath.Join(root, relative)
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name, header.ModTime = filepath.ToSlash(relative), time.Unix(0, 0)
	header.AccessTime, header.ChangeTime = time.Time{}, time.Time{}
	header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
	if info.IsDir() {
		header.Mode = 0o755
		return output.WriteHeader(header)
	}
	header.Mode = 0o444
	if err := output.WriteHeader(header); err != nil {
		return err
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	return errors.Join(copyErr, input.Close())
}

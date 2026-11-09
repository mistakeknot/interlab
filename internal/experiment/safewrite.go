package experiment

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// tempInDir creates a private temp file next to path (same directory, so the
// later rename is atomic and stays on one filesystem).
func tempInDir(path string) (*os.File, error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0644); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}

// createFileReplacing returns an open, empty file now installed at path. The
// rename replaces the directory entry, never the target of a symlink or hard
// link, so a planted link cannot redirect the write. The caller may keep
// writing to the returned file (the entry already names its inode).
func createFileReplacing(path string) (*os.File, error) {
	f, err := tempInDir(path)
	if err != nil {
		return nil, err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}

// writeFileReplacing writes data to a temp file in the same directory, syncs it
// and renames it over path. The temp file is removed on any error.
func writeFileReplacing(path string, data []byte) error {
	f, err := tempInDir(path)
	if err != nil {
		return err
	}
	tmp := f.Name()
	fail := func(err error) error {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if _, err := f.Write(data); err != nil {
		return fail(err)
	}
	if err := f.Sync(); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func checkAppendable(fi os.FileInfo, path string) error {
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file (mode %s); refusing to append", path, fi.Mode().Type())
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
		return fmt.Errorf("%s has %d hard links; refusing to append", path, st.Nlink)
	}
	return nil
}

// openAppendSafe opens path for appending, refusing symlinks, non-regular files
// and hard-linked files. The result is re-checked after open to close the race.
func openAppendSafe(path string) (*os.File, error) {
	flags := os.O_APPEND | os.O_WRONLY | syscall.O_NOFOLLOW
	pre, err := os.Lstat(path)
	switch {
	case err == nil:
		if err := checkAppendable(pre, path); err != nil {
			return nil, err
		}
	case os.IsNotExist(err):
		f, cerr := os.OpenFile(path, flags|os.O_CREATE|os.O_EXCL, 0644)
		if cerr == nil {
			return f, nil
		}
		if !os.IsExist(cerr) {
			return nil, cerr
		}
		if pre, err = os.Lstat(path); err != nil {
			return nil, err
		}
		if err := checkAppendable(pre, path); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return nil, err
	}
	post, err := f.Stat()
	if err == nil {
		err = checkAppendable(post, path)
	}
	if err == nil && !os.SameFile(pre, post) {
		err = fmt.Errorf("%s changed while opening; refusing to append", path)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

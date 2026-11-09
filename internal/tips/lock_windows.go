//go:build windows

package tips

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFile opens (creating if needed) the file at path and blocks until it
// can take an exclusive lock on it. Callers must call Unlock when done.
func lockFile(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, new(windows.Overlapped)); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &fileLock{f: f}, nil
}

// Unlock releases the lock and closes the underlying file.
func (l *fileLock) Unlock() error {
	err := windows.UnlockFileEx(windows.Handle(l.f.Fd()), 0, 1, 0, new(windows.Overlapped))
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}

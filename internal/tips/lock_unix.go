//go:build !windows

package tips

import (
	"os"
	"syscall"
)

// lockFile opens (creating if needed) the file at path and blocks until it
// can take an exclusive lock on it. Callers must call Unlock when done.
func lockFile(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &fileLock{f: f}, nil
}

// Unlock releases the lock and closes the underlying file.
func (l *fileLock) Unlock() error {
	err := syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}

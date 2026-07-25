package tips

import (
	"os"
	"syscall"
)

// fileLock is an exclusive advisory lock held on its own file, separate from
// any data file it guards. Keeping the lock on a dedicated path (rather than
// the data file itself) means the lock's identity survives the data file
// being replaced out from under it via atomic temp+rename writes.
type fileLock struct {
	f *os.File
}

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

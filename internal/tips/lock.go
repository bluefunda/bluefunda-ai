package tips

import "os"

// fileLock is an exclusive advisory lock held on its own file, separate from
// any data file it guards. Keeping the lock on a dedicated path (rather than
// the data file itself) means the lock's identity survives the data file
// being replaced out from under it via atomic temp+rename writes.
type fileLock struct {
	f *os.File
}

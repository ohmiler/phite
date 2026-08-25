package managedcomposer

import "github.com/ohmiler/phite/internal/filelock"

func acquireComposerLock(path string) (*filelock.Lock, error) {
	return filelock.Acquire(path)
}

package managedruntime

import (
	"errors"
	"os"
)

type fileLock struct {
	file *os.File
}

func acquireFileLock(path string) (*fileLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := platformLock(file); err != nil {
		file.Close()
		return nil, err
	}
	return &fileLock{file: file}, nil
}

func (lock *fileLock) Close() error {
	return errors.Join(platformUnlock(lock.file), lock.file.Close())
}

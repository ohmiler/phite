// Package filelock provides an exclusive cross-process file lock.
package filelock

import (
	"errors"
	"os"
)

type Lock struct {
	file *os.File
}

func Acquire(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := platformLock(file); err != nil {
		file.Close()
		return nil, err
	}
	return &Lock{file: file}, nil
}

func (lock *Lock) Close() error {
	return errors.Join(platformUnlock(lock.file), lock.file.Close())
}

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// acquireLock locks the database directory to prevent multiple processes from accessing it.
func acquireLock(dataDir string) (*os.File, error) {
	lockPath := filepath.Join(dataDir, "lock.db")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	// LOCK_EX = exclusive lock
	// LOCK_NB = non-blocking (fail immediately if already locked)
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		return nil, ErrDatabaseLocked
	}

	return file, nil
}

// releaseLock unlocks the database and closes/removes the lock file.
func releaseLock(file *os.File) error {
	if file == nil {
		return nil
	}
	defer file.Close()

	// Unlock file descriptor
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	// Remove lock file from directory
	_ = os.Remove(file.Name())
	return nil
}

package main

import (
	"fmt"
	"time"

	"github.com/btnmasher/random"
)

func AcquireLock(entry *Entry, timeout time.Duration, newid string) error {
	minder := time.NewTicker(timeout)
	defer minder.Stop()

	rchan := make(chan error, 1)
	r := &WriteAction{
		Entry: entry,
		Value: newid,
		Error: rchan,
	}

	acquireLock <- r

	select {
	case err := <-r.Error:
		return err
	case <-minder.C:
		timeoutLock <- AcquireAction{Key: entry.GetKey(), Id: newid}
		return fmt.Errorf("Timed out waiting for lock acquisition.")
	}
}

func newLockId() string {
	newid := random.String(5)
	logger.Debugf("New LockId generated: %s", newid)

	//Ensure we don't collide our LockId.
	for locks.LockExists(newid) {
		newid = random.String(5)
		logger.Debugf("Collision occured, new LockId generated: %s", newid)
	}

	return newid
}

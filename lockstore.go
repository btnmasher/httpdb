package main

import (
	"fmt"
	"sync"
)

type LockStore struct {
	sync.Mutex
	Locks map[string]struct{}
}

func (l *LockStore) LockExists(id string) bool {
	l.Lock()
	defer l.Unlock()
	_, exists := l.Locks[id]
	return exists
}

func (l *LockStore) AddLock(id string) error {
	l.Lock()
	defer l.Unlock()
	if _, exists := l.Locks[id]; exists {
		return fmt.Errorf("Cannot add lock id '%s', already exists.", id)
	} else {
		l.Locks[id] = struct{}{}
		return nil
	}
}

func (l *LockStore) DeleteLock(id string) error {
	l.Lock()
	defer l.Unlock()
	if _, exists := l.Locks[id]; !exists {
		return fmt.Errorf("Cannot delete lock '%s', does not exist.", id)
	} else {
		delete(l.Locks, id)
		return nil
	}
}

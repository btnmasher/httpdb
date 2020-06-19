package main

import (
	"fmt"
	"sync"
)

type DataStore struct {
	sync.Mutex
	Entries map[string]*Entry
}

func (d *DataStore) EntryExists(key string) bool {
	d.Lock()
	defer d.Unlock()
	_, exists := d.Entries[key]
	return exists
}

func (d *DataStore) AddEntry(entry *Entry) error {
	d.Lock()
	defer d.Unlock()
	if _, exists := d.Entries[entry.Key]; exists {
		return fmt.Errorf("Cannot add entry '%s', key already exists.", entry.Key)
	} else {
		d.Entries[entry.Key] = entry
		return nil
	}
}

func (d *DataStore) NewEntry(key string) (*Entry, error) {
	d.Lock()
	defer d.Unlock()
	if _, exists := d.Entries[key]; exists {
		return nil, fmt.Errorf("Cannot create new entry '%s', key already exists.", key)
	} else {
		entry := &Entry{Key: key}
		d.Entries[entry.Key] = entry
		return entry, nil
	}
}

func (d *DataStore) GetEntry(key string) (*Entry, error) {
	d.Lock()
	defer d.Unlock()
	if _, exists := d.Entries[key]; !exists {
		return nil, fmt.Errorf("Cannot get entry '%s', does not exist.", key)
	} else {
		return d.Entries[key], nil
	}
}

func (d *DataStore) DeleteEntry(key string) error {
	d.Lock()
	defer d.Unlock()
	if entry, exists := d.Entries[key]; !exists {
		return fmt.Errorf("Cannot delete entry '%s', does not exist.", key)
	} else {
		delete(d.Entries, key)
		locks.DeleteLock(entry.GetLockId())
		deleteEntry <- AcquireAction{Key: key}
		return nil
	}
}

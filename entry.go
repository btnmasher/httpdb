package main

import (
	"encoding/json"
	"sync"
)

type Entry struct {
	sync.Mutex
	Key    string `json:"-"`
	Value  string `json:"value"`
	LockId string `json:"lock_id"`
}

func (e *Entry) IsLocked() bool {
	// e.Lock()
	// defer e.Unlock()
	return e.LockId != ""
}

func (e *Entry) IsLockedAtomic() bool {
	logger.Debug("Running atomic.")
	rchan := make(chan bool, 1)
	defer close(rchan)

	r := &BoolResponder{
		Entry:  e,
		Return: rchan,
	}

	isLocked <- r

	return <-rchan
}

func (e *Entry) SetLockId(id string) bool {
	// e.Lock()
	// defer e.Unlock()
	if e.LockId != "" {
		return false
	} else {
		e.LockId = id
		return true
	}
}

func (e *Entry) SetLockIdAtomic(id string) bool {
	logger.Debug("Running atomic.")
	echan := make(chan error, 1)
	defer close(echan)

	r := &WriteAction{
		Entry: e,
		Error: echan,
		Value: id,
	}

	setLockId <- r

	err := <-echan

	logger.Debug(err)

	return err == nil
}

func (e *Entry) ValidLock(id string) bool {
	// e.Lock()
	// defer e.Unlock()
	return e.LockId == id
}

func (e *Entry) ValidLockAtomic(id string) bool {
	logger.Debug("Running atomic.")
	rchan := make(chan bool, 1)
	defer close(rchan)

	r := &BoolResponder{
		Entry:  e,
		Param:  id,
		Return: rchan,
	}

	validLock <- r

	return <-rchan
}

func (e *Entry) UnsetLockId() {
	// e.Lock()
	// defer e.Unlock()
	locks.DeleteLock(e.LockId)
	e.LockId = ""
	releaseLock <- AcquireAction{Key: e.Key}
}

func (e *Entry) UnsetLockIdAtomic() {
	logger.Debug("Running atomic.")
	echan := make(chan error, 1)
	defer close(echan)

	a := &WriteAction{
		Entry: e,
		Error: echan,
	}

	unsetLockId <- a

	<-echan

	return
}

func (e *Entry) GetLockId() string {
	// e.Lock()
	// defer e.Unlock()
	return e.LockId
}

func (e *Entry) GetLockIdAtomic() string {
	logger.Debug("Running atomic.")
	rchan := make(chan string, 1)
	defer close(rchan)

	r := &StringResponder{
		Entry:  e,
		Return: rchan,
	}

	getLockId <- r

	return <-rchan
}

func (e *Entry) SetValue(value string) {
	// e.Lock()
	// defer e.Unlock()
	e.Value = value
}

func (e *Entry) SetValueAtomic(value string) {
	logger.Debug("Running atomic.")
	echan := make(chan error, 1)
	defer close(echan)

	a := &WriteAction{
		Entry: e,
		Value: value,
		Error: echan,
	}

	setValue <- a

	<-echan

	return
}

func (e *Entry) GetValue() string {
	// e.Lock()
	// defer e.Unlock()
	return e.Value
}

func (e *Entry) GetValueAtomic() string {
	logger.Debug("Running atomic.")
	rchan := make(chan string, 1)
	defer close(rchan)

	r := &StringResponder{
		Entry:  e,
		Return: rchan,
	}

	getValue <- r

	return <-rchan
}

func (e *Entry) GetKey() string {
	// e.Lock()
	// defer e.Unlock()
	return e.Key
}

func (e *Entry) GetKeyAtomic() string {
	logger.Debug("Running atomic.")
	rchan := make(chan string, 1)
	defer close(rchan)

	r := &StringResponder{
		Entry:  e,
		Return: rchan,
	}

	getKey <- r

	return <-rchan
}

func (e *Entry) GetJson() ([]byte, error) {
	// e.Lock()
	// defer e.Unlock()
	j, err := json.Marshal(e)
	if err != nil {
		return []byte{}, err
	}
	return j, nil
}

func (e *Entry) GetJsonAtomic() ([]byte, error) {
	logger.Debug("Running atomic.")
	rchan := make(chan []byte, 1)
	defer close(rchan)

	echan := make(chan error, 1)
	defer close(echan)

	r := &ByteResponder{
		Entry:  e,
		Return: rchan,
		Error:  echan,
	}

	getJson <- r

	select {
	case bytes := <-rchan:
		return bytes, nil
	case err := <-echan:
		return []byte{}, err
	}
}

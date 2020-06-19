package main

import (
	"container/list"
	"fmt"
)

var (
	isLocked    chan *BoolResponder   = make(chan *BoolResponder, Config.App.AtomicBuffer)
	validLock   chan *BoolResponder   = make(chan *BoolResponder, Config.App.AtomicBuffer)
	unsetLockId chan *WriteAction     = make(chan *WriteAction, Config.App.AtomicBuffer)
	setLockId   chan *WriteAction     = make(chan *WriteAction, Config.App.AtomicBuffer)
	getLockId   chan *StringResponder = make(chan *StringResponder, Config.App.AtomicBuffer)
	getValue    chan *StringResponder = make(chan *StringResponder, Config.App.AtomicBuffer)
	setValue    chan *WriteAction     = make(chan *WriteAction, Config.App.AtomicBuffer)
	getKey      chan *StringResponder = make(chan *StringResponder, Config.App.AtomicBuffer)
	getJson     chan *ByteResponder   = make(chan *ByteResponder, Config.App.AtomicBuffer)
	acquireLock chan *WriteAction     = make(chan *WriteAction, Config.App.AtomicBuffer)
	releaseLock chan AcquireAction    = make(chan AcquireAction, Config.App.AtomicBuffer)
	timeoutLock chan AcquireAction    = make(chan AcquireAction, Config.App.AtomicBuffer)
	deleteEntry chan AcquireAction    = make(chan AcquireAction, Config.App.AtomicBuffer)
)

type AcquireAction struct {
	Key string
	Id  string
}

type WriteAction struct {
	Entry *Entry
	Value string
	Error chan error
}

type BoolResponder struct {
	Entry  *Entry
	Param  string
	Return chan bool
}

type StringResponder struct {
	Entry  *Entry
	Param  string
	Return chan string
	Error  chan error
}

type ByteResponder struct {
	Entry  *Entry
	Return chan []byte
	Error  chan error
}

func startAtomics(stop chan struct{}) {
	logger.Info("Started Atomics Gouroutine.")
	for {
		select {

		case il := <-isLocked:
			logger.Debug("Read isLocked channel.")
			il.Return <- il.Entry.IsLocked()

		case vl := <-validLock:
			logger.Debug("Read validLock channel.")
			vl.Return <- vl.Entry.ValidLock(vl.Param)

		case ul := <-unsetLockId:
			logger.Debug("Read unsetLockId channel.")
			ul.Entry.UnsetLockId()
			ul.Error <- nil

		case cl := <-setLockId:
			logger.Debug("Read setLockId channel.")
			if cl.Entry.SetLockId(cl.Value) {
				cl.Error <- nil
			} else {
				cl.Error <- fmt.Errorf("Could not set new LockId, already locked: %s", cl.Entry.GetKey())
			}

		case gl := <-getLockId:
			logger.Debug("Read getLockId channel.")
			gl.Return <- gl.Entry.GetLockId()

		case gv := <-getValue:
			logger.Debug("Read getValue channel.")
			gv.Return <- gv.Entry.GetValue()

		case sv := <-setValue:
			logger.Debug("Read setValue channel.")
			sv.Entry.SetValue(sv.Value)
			sv.Error <- nil

		case gk := <-getKey:
			logger.Debug("Read getKey channel.")
			gk.Return <- gk.Entry.GetKey()

		case gj := <-getJson:
			logger.Debug("Read getJson channel.")
			j, err := gj.Entry.GetJson()
			if err != nil {
				gj.Error <- err
				break
			}
			gj.Return <- j

		case <-stop:
			logger.Info("Stopped Atomics Goroutine.")
			return

		}
	}
}

func startLockMinder(stop chan struct{}) {
	lockers := make(map[string]*list.List) //Map of all the waiting lockers.

	logger.Info("Started Lock Minder Gouroutine.")
	for {
		select {

		case a := <-acquireLock:
			logger.Debug("Read acquireLock channel.")

			key := a.Entry.GetKey()

			//Check our map if we have a list of waiting lockers already.
			if locks, exists := lockers[key]; exists {
				logger.Debugf("Found waitlist for key: %s - Adding lockId: %s", key, a.Value)

				//Found list of waiting lockers, add this one.
				locks.PushBack(a)
			} else {

				//Make a new list of waiting lockers for this key.
				l := list.New()
				l.PushBack(a)
				lockers[key] = l
			}

		case r := <-releaseLock:
			logger.Debugf("Read releaseLock channel: %v", r)

			//For each release of a lock, check if we have a waiting locker trying to acquire.
			if locks, exists := lockers[r.Key]; exists {
				logger.Debugf("Found waitlist for key: %s", r.Key)

				//Make sure we don't out-of-bounds because we're paranoid
				if locks.Len() > 0 {

					e := locks.Front()

					//Send the WriteAction to the atomic setter and remove the waiting locker.
					logger.Debugf("Setting lockId for entry: %s to value: %s", r.Key, e.Value.(*WriteAction).Value)
					setLockId <- e.Value.(*WriteAction)
					locks.Remove(e)

					//Clean up the list if we're emptry
					if locks.Len() == 0 {
						logger.Debugf("Cleaning waitlist for key: %s", r.Key)
						delete(lockers, r.Key)
					}

				} else {
					logger.Debugf("Cleaning waitlist for key: %s", r.Key)

					//Somehow didn't clean up the list of lockers, this shouldn't happen
					delete(lockers, r.Key)
					logger.Debug(lockers)
				}
			}

		case t := <-timeoutLock:
			logger.Debugf("Read timeoutLock channel: %v", t)

			//Check our map if we have a list of waiting lockers
			if locks, exists := lockers[t.Key]; exists {
				logger.Debugf("Found waitlist for key: %s", t.Key)

				e := locks.Front()

				//Loop and find our particular waiting locker by LockId.
				for i := 0; i < locks.Len(); i++ {
					if e.Value.(*WriteAction).Value == t.Id {
						logger.Debugf("Removing locker from waitlist: %s", t.Id)

						//Found the locker, remove it from the list.
						locks.Remove(e)

						//Clean up the list if we're emptry
						if locks.Len() == 0 {
							delete(lockers, t.Key)
						}

						break
					}
					e = e.Next()
					if e == nil {
						logger.Debugf("Prematurely hit end of the linked list when removing locker from waitlist: %s", t.Id)
						break
					}
				}
			}

		case d := <-deleteEntry:
			logger.Debug("Read deleteEntry channel")

			//Check our map if we have a list of waiting lockers
			if locks, exists := lockers[d.Key]; exists {
				logger.Debugf("Found waitlist for key: %s", d.Key)

				e := locks.Front()

				//Loop and find our particular waiting locker by LockId.
				for i := 0; i < locks.Len(); i++ {
					e.Value.(*WriteAction).Error <- fmt.Errorf("Entry was deleted before lock could be acquired.")
					e = e.Next()
					if e == nil {
						logger.Debugf("Prematurely hit end of the linked list when removing locker from waitlist: %s", d.Id)
						break
					}
				}
				locks.Init()
				delete(lockers, d.Key)

			}

		case <-stop:
			logger.Info("Stopped Lock Minder Goroutine.")
			return

		}
	}
}

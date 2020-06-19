package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/btnmasher/random"
	"github.com/gorilla/mux"
)

func reserveKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	logger.Infof("Received PUT request to /reservations/{key}, request id: %s", random.String(5))
	logger.Debugf("Received varaibles: %v", vars)

	//Check to see if we actually got a key.
	key, exists := vars["key"]
	if !exists {
		logger.Info("Invalid request, no key specified.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//Get the reference to the entry for the specified key.
	entry, err := data.GetEntry(key)
	if err != nil {
		logger.Infof("Invalid request, entry key not found: %s", key)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	entry.Lock()
	defer entry.Unlock()

	newid := newLockId()

	//Check the LockId, if unlocked, set lock. If locked, acquire lock.
	if entry.SetLockId(newid) {
		logger.Debug("Set the lock successfully.")
	} else {
		//Looks like someone has it already, attempt an acquisition.
		err := AcquireLock(entry, time.Second*Config.App.TimeOut, newid)
		if err != nil {
			logger.Info(err)
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}
		logger.Debug("Acquired the lock successfully.")
	}

	j, err := entry.GetJson()

	if err != nil {
		logger.Errorf("Error marshaling entry to json: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(j)
	logger.Infof("Handled successful request for: %s", key)
}

func updateVal(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	logger.Infof("Received PUT request to /values/{key}/{lock_id}, request id: %s", random.String(5))
	logger.Debugf("Received varaibles: %v", vars)

	//Check to see if we actually got a key.
	key, exists := vars["key"]
	if !exists {
		logger.Info("Invalid request, no key specified.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//Get the reference to the entry.
	entry, err := data.GetEntry(key)
	if err != nil {
		logger.Infof("Invalid request, entry key not found: %s", key)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	entry.Lock()
	defer entry.Unlock()

	//Check to make sure we got a LockId specified.
	lockid, exists := vars["lock_id"]
	if !exists {
		logger.Info("Invalid request, no lock_id specified.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//Parse the query value, treat an empty query value as if "false".
	rel := r.FormValue("release")
	release := false
	if rel == "true" {
		release = true
		logger.Debug("Release set to true.")
	} else if rel != "" && rel != "false" {
		logger.Infof("Invalid release query specified: %s", rel)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//Check the LockId.
	logger.Debugf("Checking lock validity for entry: %s - LockId: %s", key, lockid)
	if !entry.ValidLock(lockid) {
		logger.Debugf("LockId does not match entry: %s - LockId: %s - Expected: %s", key, lockid, entry.GetLockId())
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	//If we made it here, it means we either didn't have a lock
	//or the correct LockId was specified.
	logger.Debug("LockId matches, reading new value.")

	//Get the new value.
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("Error reading request body: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	entry.SetValue(string(bytes))
	logger.Info("Successfully set new submitted value.")
	w.WriteHeader(http.StatusNoContent)

	if release {
		logger.Infof("Removing lock from entry: %s", entry.GetKey())
		entry.UnsetLockId()
	}
	logger.Infof("Handled successful request for: %s", key)
}

func putVal(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	logger.Infof("Received PUT request to /values/{key}, request id: %s", random.String(5))
	logger.Debugf("Received varaibles: %v", vars)

	//Check to see if we actually got a key.
	key, exists := vars["key"]
	if !exists {
		logger.Info("Invalid request, no key specified.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//Get the reference to the entry if it exists.
	entry, err := data.GetEntry(key)
	if err != nil {
		logger.Infof("Generating new entry for key: %s", key)
		logger.Debugf("Received during GetEntry: %s", err)

		//Didn't exist, make a new one!
		entry = &Entry{Key: key}
		err = data.AddEntry(entry)
		if err != nil {
			logger.Debug(err)
		}
	}

	entry.Lock()
	defer entry.Unlock()

	logger.Debug("Reading request body... ")
	//Get the new value.
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("Error occured reading request body: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Debugf("Received request body: %s", string(bytes))

	newid := newLockId()

	logger.Debug("Checking entry lock state.")

	//Check the LockId, if unlocked, set lock. If locked, acquire lock.
	if entry.SetLockId(newid) {
		logger.Debug("Set the lock successfully.")
	} else {
		//Looks like someone has it already, attempt an acquisition.
		err := AcquireLock(entry, time.Second*Config.App.TimeOut, newid)
		if err != nil {
			logger.Info(err)
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}
		logger.Debug("Acquired the lock successfully.")
	}

	entry.SetValue(string(bytes))

	//Marhsal just the LockId into json and return it.
	j, err := json.Marshal(map[string]string{"lock_id": newid})
	if err != nil {
		logger.Errorf("Error unmarshaling JSON for key: %s: %s", key, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(j)
	logger.Infof("Handled successful request for: %s", key)
}

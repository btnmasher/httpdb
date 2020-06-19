package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/btnmasher/lumberjack"
	"github.com/btnmasher/random"
	"github.com/gorilla/mux"
)

var (
	postResUrl     string
	postValUrl     string
	putValUrl      string
	recvCodeErrMsg string
	muxr           *mux.Router
)

func init() {
	postResUrl = "/reservations/%s"
	putValUrl = "/values/%s"
	postValUrl = "/values/%s/%s?release=%s"
	recvCodeErrMsg = "Should have received %v. Received: %v"
	muxr = mux.NewRouter()
	regHandlers(muxr)

	logger = lumberjack.NewLogger() //Turn off the logger (empty: no backends, no levels)
	//logger.AddLevel(lumberjack.DEBUG) //Leave the logger on and enable DEBUG

	done := make(chan struct{})
	go startAtomics(done)
	go startLockMinder(done)

	Config.App.TimeOut = 1
}

func TestReserveKeyNoExists(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)

	req, err := http.NewRequest("POST", fmt.Sprintf(postResUrl, testKey), nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()
	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusNotFound, w.Code)
}

func TestPutValNoExists(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)
	testVal := random.String(10)

	testdata := strings.NewReader(testVal)
	req, err := http.NewRequest("PUT", fmt.Sprintf(putValUrl, testKey), testdata)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusOK, w.Code)

	if !data.EntryExists(testKey) {
		t.Error("Data should exist.")
	}

	entry, err := data.GetEntry(testKey)
	if err != nil {
		t.Errorf("Error getting data entry from key: %s", err)
	}

	if !entry.IsLocked() {
		t.Error("Entry should be locked.")
	}

	if entry.GetValue() != testVal {
		t.Error("Expected data incorrect.")
	}

	val := make(map[string]string)
	rdata := w.Body.Bytes()

	err = json.Unmarshal(rdata, &val)
	if err != nil {
		t.Errorf("Unmarshal error: %s", err)
	}

	if l, ok := val["lock_id"]; !ok {
		t.Error("Did not receive valid JSON data.")
	} else {
		if !entry.ValidLock(l) {
			t.Error("Entry should be locked with expected LockId.")
		}
	}
}

func TestPutValExistsLockedAcquire(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)
	testVal := random.String(10)
	testLockId := random.String(5)

	err := data.AddEntry(&Entry{Key: testKey, Value: testVal, LockId: testLockId})
	if err != nil {
		t.Error(err)
	}

	testNewVal := "NewValue"
	testdata := strings.NewReader(testNewVal)
	req, err := http.NewRequest("PUT", fmt.Sprintf(putValUrl, testKey), testdata)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	if !data.EntryExists(testKey) {
		t.Error("Data should exist.")
	}

	entry, err := data.GetEntry(testKey)
	if err != nil {
		t.Errorf("Error getting data entry from key: %s", err)
	}

	tick := time.NewTicker(time.Millisecond * 100)
	go func() {

		if !entry.IsLocked() {
			t.Error("Entry should be locked.")
		}

		if entry.GetValue() == testNewVal {
			t.Error("Data should still be incorrect before acquisition of lock.")
		}

		<-tick.C

		entry.UnsetLockId()
		if entry.IsLocked() {
			t.Error("Entry should no longer be locked.")
		}

		<-tick.C
	}()

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusOK, w.Code)

	if !entry.IsLocked() {
		t.Error("Entry should be locked.")
	}

	if entry.GetValue() != testNewVal {
		t.Error("Expected data incorrect.")
	}

	tick.Stop()

	val := make(map[string]string)
	rdata := w.Body.Bytes()

	err = json.Unmarshal(rdata, &val)
	if err != nil {
		t.Errorf("Unmarshal error: %s", err)
	}

	if l, ok := val["lock_id"]; !ok {
		t.Error("Did not receive valid JSON data.")
	} else {
		testLockId = l
	}

	if !entry.ValidLock(testLockId) {
		t.Error("Entry should be locked with expected LockId.")
	}
}

func TestPutValExistsLocked(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)
	testVal := random.String(10)
	testLockId := random.String(5)

	err := data.AddEntry(&Entry{Key: testKey, Value: testVal, LockId: testLockId})
	if err != nil {
		t.Error(err)
	}

	testdata := strings.NewReader(testVal)
	req, err := http.NewRequest("PUT", fmt.Sprintf(putValUrl, testKey), testdata)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusRequestTimeout, w.Code)

	if !data.EntryExists(testKey) {
		t.Error("Data should exist.")
	}

	entry, err := data.GetEntry(testKey)
	if err != nil {
		t.Errorf("Error getting data entry from key: %s", err)
	}

	if !entry.IsLocked() {
		t.Error("Entry should be locked.")
	}

	if !entry.ValidLock(testLockId) {
		t.Error("Entry should be locked with expected LockId.")
	}

	if entry.GetValue() != testVal {
		t.Error("Expected data incorrect.")
	}
}

func TestUpdateValNoExists(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testVal := random.String(10)

	testdata := strings.NewReader(testVal)
	req, err := http.NewRequest("POST", fmt.Sprintf(postValUrl, "invalidkey", "invalidlockid", "false"), testdata)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusNotFound, w.Code)
}

func TestUpdateValExistsInvalidLockRelease(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)
	testVal := random.String(10)
	testLockId := random.String(5)

	err := data.AddEntry(&Entry{Key: testKey, Value: testVal, LockId: testLockId})
	if err != nil {
		t.Error(err)
	}

	testdata := strings.NewReader(testVal)
	req, err := http.NewRequest("POST", fmt.Sprintf(postValUrl, testKey, "invalidlock", "true"), testdata)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusUnauthorized, w.Code)

	entry, err := data.GetEntry(testKey)
	if err != nil {
		t.Errorf("Error getting data entry from key: %s", err)
	}

	if !entry.IsLocked() {
		t.Error("Entry should be locked.")
	}

	if entry.GetValue() != testVal {
		t.Error("Expected data incorrect.")
	}
}

func TestUpdateValExistsLockedValidLockRelease(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)
	testVal := random.String(10)
	testLockId := random.String(5)

	err := data.AddEntry(&Entry{Key: testKey, Value: testVal, LockId: testLockId})
	if err != nil {
		t.Error(err)
	}

	testNewVal := "NewTestValue"
	testdata := strings.NewReader(testNewVal)
	req, err := http.NewRequest("POST", fmt.Sprintf(postValUrl, testKey, testLockId, "true"), testdata)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusNoContent, w.Code)

	entry, err := data.GetEntry(testKey)
	if err != nil {
		t.Errorf("Error getting data entry from key: %s", err)
	}

	if entry.IsLocked() {
		t.Error("Entry should not be locked.")
	}

	if entry.GetValue() != testNewVal {
		t.Error("Expected data incorrect.")
	}
}

func TestUpdateValExistsUnlocked(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)
	testVal := random.String(10)

	err := data.AddEntry(&Entry{Key: testKey, Value: testVal})
	if err != nil {
		t.Error(err)
	}

	testdata := strings.NewReader(testVal)
	req, err := http.NewRequest("POST", fmt.Sprintf(postValUrl, testKey, "invalidlock", "false"), testdata)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	entry, err := data.GetEntry(testKey)
	if err != nil {
		t.Errorf("Error getting data entry from key: %s", err)
	}

	if entry.IsLocked() {
		t.Error("Entry should be unlocked.")
	}

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusUnauthorized, w.Code)

	if entry.IsLocked() {
		t.Error("Entry should still be unlocked.")
	}

	if entry.GetValue() != testVal {
		t.Error("Expected data incorrect.")
	}
}

func TestReserveKeyExistsUnlockedTimeout(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)
	testVal := random.String(10)

	err := data.AddEntry(&Entry{Key: testKey, Value: testVal})
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf(postResUrl, testKey), nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusOK, w.Code)

	val := make(map[string]string)
	rdata := w.Body.Bytes()

	err = json.Unmarshal(rdata, &val)
	if err != nil {
		t.Errorf("Unmarshal error: %s", err)
	}

	entry, err := data.GetEntry(testKey)
	if err != nil {
		t.Errorf("Error getting data entry from key: %s", err)
	}

	if !entry.IsLocked() {
		t.Error("Entry should be locked.")
	}

	if l, ok := val["lock_id"]; !ok {
		t.Error("Did not receive valid JSON data.")
	} else {
		if !entry.ValidLock(l) {
			t.Error("Received data should match expected LockId.")
		}
	}

	if l, ok := val["value"]; !ok {
		t.Error("Did not receive valid JSON data.")
	} else {
		if l != testVal {
			t.Error("Received data should match expected value.")
		}
	}
}

func TestReserveKeyExistsLockedTimeout(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)
	testVal := random.String(10)
	testLockId := random.String(5)

	req, err := http.NewRequest("POST", fmt.Sprintf(postResUrl, testKey), nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	err = data.AddEntry(&Entry{Key: testKey, Value: testVal, LockId: testLockId})
	if err != nil {
		t.Error(err)
	}

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusRequestTimeout, w.Code)

	entry, err := data.GetEntry(testKey)
	if err != nil {
		t.Errorf("Error getting data entry from key: %s", err)
	}

	if !entry.IsLocked() {
		t.Error("Entry should be locked.")
	}

	if !entry.ValidLock(testLockId) {
		t.Error("Entry should still be locked with expected LockId.")
	}

	if entry.GetValue() != testVal {
		t.Error("Expected data incorrect.")
	}
}

func TestReserveKeyExistsLockedAcquire(t *testing.T) {
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
	testKey := random.String(5)
	testVal := random.String(10)
	testLockId := random.String(5)

	req, err := http.NewRequest("POST", fmt.Sprintf(postResUrl, testKey), nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	err = data.AddEntry(&Entry{Key: testKey, Value: testVal, LockId: testLockId})
	if err != nil {
		t.Error(err)
	}

	if !data.EntryExists(testKey) {
		t.Error("Data should exist.")
	}

	entry, err := data.GetEntry(testKey)
	if err != nil {
		t.Errorf("Error getting data entry from key: %s", err)
	}

	tick := time.NewTicker(time.Millisecond * 100)
	go func() {

		if !entry.IsLocked() {
			t.Error("Entry should be locked.")
		}

		<-tick.C

		entry.UnsetLockId()
		if entry.IsLocked() {
			t.Error("Entry should no longer be locked.")
		}

		<-tick.C
	}()

	muxr.ServeHTTP(w, req)
	checkCode(t, http.StatusOK, w.Code)

	val := make(map[string]string)
	rdata := w.Body.Bytes()

	err = json.Unmarshal(rdata, &val)
	if err != nil {
		t.Errorf("Unmarshal error: %s", err)
	}

	if !entry.IsLocked() {
		t.Error("Entry should be locked.")
	}

	testLockId = entry.GetLockId()

	if l, ok := val["lock_id"]; !ok {
		t.Error("Did not receive valid JSON data.")
	} else {
		if !entry.ValidLock(l) {
			t.Error("Received data should match expected LockId.")
		}
	}

	if l, ok := val["value"]; !ok {
		t.Error("Did not receive valid JSON data.")
	} else {
		if l != entry.GetValue() {
			t.Error("Received data should match expected value.")
		}
	}
}

func checkCode(t *testing.T, expected, received int) {
	if received != expected {
		t.Errorf(recvCodeErrMsg, expected, received)
	}
}

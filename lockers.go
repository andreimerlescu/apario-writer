package main

import (
	"log"
	"sync"
)

// Checksum Locker
var cslMu *sync.RWMutex
var checksumLockers map[string]*sync.RWMutex
var ChecksumLocker = func(checksum string) *sync.RWMutex {
	cslMu.Lock()
	log.Printf("ChecksumLocker(%s) Locked...", checksum)
	defer func() {
		log.Printf("... ChecksumLocker(%s) Unlocked!", checksum)
		dlMu.Unlock()
	}()
	defer cslMu.Unlock()
	if _, found := checksumLockers[checksum]; !found {
		checksumLockers[checksum] = &sync.RWMutex{}
	}
	return checksumLockers[checksum]
}

// Page Locker
var plMu *sync.RWMutex
var pageLockers map[string]*sync.RWMutex
var PageLocker = func(id string) *sync.RWMutex {
	plMu.Lock()
	log.Printf("PageLocker(%s) Locked...", id)
	defer func() {
		log.Printf("... PageLocker(%s) Unlocked!", id)
		dlMu.Unlock()
	}()
	defer plMu.Unlock()
	if _, found := pageLockers[id]; !found {
		pageLockers[id] = &sync.RWMutex{}
	}
	return pageLockers[id]
}

// Document Locker
var dlMu *sync.RWMutex
var documentLockers map[string]*sync.RWMutex
var DocumentLocker = func(id string) *sync.RWMutex {
	dlMu.Lock()
	log.Printf("DocumentLocker(%s) Locked...", id)
	defer func() {
		log.Printf("... DocumentLocker(%s) Unlocked!", id)
		dlMu.Unlock()
	}()
	if _, found := documentLockers[id]; !found {
		documentLockers[id] = &sync.RWMutex{}
	}
	return documentLockers[id]
}

func init() {
	// Checksums
	cslMu = &sync.RWMutex{}
	checksumLockers = make(map[string]*sync.RWMutex)
	// Documents
	dlMu = &sync.RWMutex{}
	documentLockers = make(map[string]*sync.RWMutex)
	// Pages
	pageLockers = make(map[string]*sync.RWMutex)
	plMu = &sync.RWMutex{}
}

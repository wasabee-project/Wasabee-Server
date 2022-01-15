package util

import (
	"sync"
)

// A safemap is a special-purpose map type for holding unsigned-ints or bools
// intended for use in http requests or performance-critical paths
type Safemap struct {
	d map[string]uint64
	m sync.RWMutex
}

// Get returns a uint64 value, if it exists in the map
func (sm *Safemap) Get(key string) (uint64, bool) {
	sm.m.RLock()
	value, ok := sm.d[key]
	sm.m.RUnlock()
	return value, ok
}

// Set sets a uint64 value into the map
func (sm *Safemap) Set(key string, value uint64) {
	sm.m.Lock()
	sm.d[key] = value
	sm.m.Unlock()
}

// Increment automatically increments the value in the map
func (sm *Safemap) Increment(key string) {
	sm.m.Lock()
	v, ok := sm.d[key]
	if ok {
		sm.d[key] = v + 1
	} else {
		sm.d[key] = 1
	}
	sm.m.Unlock()
}

// GetBool returns a bool value from the map -- false for missing key
func (sm *Safemap) GetBool(key string) bool {
	sm.m.RLock()
	value, ok := sm.d[key]
	sm.m.RUnlock()
	return ok && value > 0
}

// SetBook sets a boolean value into the map
func (sm *Safemap) SetBool(key string, value bool) {
	sm.m.Lock()
	if !value {
		delete(sm.d, key)
	} else {
		sm.d[key] = 1
	}
	sm.m.Unlock()
}

// NewSafeMap returns an initialized pointer to a Safemap
func NewSafemap() *Safemap {
	var n Safemap
	n.d = make(map[string]uint64)
	return &n
}

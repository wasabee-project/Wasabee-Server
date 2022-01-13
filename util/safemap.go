package util

import (
	"sync"
)

type Safemap struct {
	d map[string]uint64
	m sync.RWMutex
}

func (sm *Safemap) Get(key string) (uint64, bool) {
	sm.m.RLock()
	value, ok := sm.d[key]
	sm.m.RUnlock()
	return value, ok
}

func (sm *Safemap) Set(key string, value uint64) {
	sm.m.Lock()
	sm.d[key] = value
	sm.m.Unlock()
}

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

func (sm *Safemap) GetBool(key string) bool {
	sm.m.RLock()
	value, ok := sm.d[key]
	sm.m.RUnlock()
	return ok && value > 0
}

func (sm *Safemap) SetBool(key string, value bool) {
	sm.m.Lock()
	if !value {
		delete(sm.d, key)
	} else {
		sm.d[key] = 1
	}
	sm.m.Unlock()
}

func NewSafemap() *Safemap {
	var n Safemap
	n.d = make(map[string]uint64)
	return &n
}

package main

import (
	"sync"
	"time"
)

var (
	lockerMutex sync.Mutex
	locker      = make(map[string]*lockerUnit)
)

type lockerUnit struct {
	l sync.Mutex
	n int
	t time.Time
}

func Lock(file string) {
	var lu *lockerUnit
	var ok bool
	lockerMutex.Lock()
	if lu, ok = locker[file]; ok {
		lu.t = time.Now()
		lu.n = lu.n + 1
	} else {
		lu = &lockerUnit{t: time.Now(), n: 1}
		locker[file] = lu
	}
	lockerMutex.Unlock()
	lu.l.Lock()
}

func Unlock(file string) {
	lockerMutex.Lock()
	if lu, ok := locker[file]; ok {
		lu.t = time.Now()
		lu.n = lu.n - 1
		if lu.n == 0 {
			delete(locker, file)
		}
		lu.l.Unlock()
	}
	lockerMutex.Unlock()
}

func init() {
	go func() {
		for {
			time.Sleep(time.Hour)
			lockerMutex.Lock()
			for key, lu := range locker {
				if lu.n == 0 && time.Now().Sub(lu.t) > time.Hour {
					delete(locker, key)
				}
			}
			lockerMutex.Unlock()
		}
	}()
}

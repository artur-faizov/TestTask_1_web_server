package main

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

type MapDB struct {
	lastID  int32
	History map[int32]HistoryElement
	mux     *sync.RWMutex
}

func NewMapDB() *MapDB {
	return &MapDB{
		lastID:  int32(0),
		History: make(map[int32]HistoryElement),
		mux:     &sync.RWMutex{},
	}
}

func (db *MapDB) Add(newHistoryElement HistoryElement) error {
	x := atomic.AddInt32(&db.lastID, 1)
	db.mux.Lock()
	db.History[x] = newHistoryElement
	db.mux.Unlock()
	return nil
}

func (db *MapDB) Delete(id int32) error {
	db.mux.Lock()
	delete(db.History, id)
	db.mux.Unlock()
	return nil
}

type ByTime []*historyCopyElement

func (a ByTime) Len() int           { return len(a) }
func (a ByTime) Less(i, j int) bool { return a[i].Element.Time.Before(a[j].Element.Time) }
func (a ByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func (db *MapDB) GetHistory(offset, limit int) ([]*historyCopyElement, error) {

	if offset > len(db.History) {
		return nil, fmt.Errorf("offset %d greater than size of DB %d", offset, len(db.History))
	}
	db.mux.RLock()

	historyCopy := make([]*historyCopyElement, 0)

	for key, element := range db.History {
		historyCopy = append(historyCopy, &historyCopyElement{ID: key, Element: element})
	}
	db.mux.RUnlock()

	from := offset
	to := len(historyCopy)
	if limit != 0 && limit+offset < to {
		to = offset + limit
	}

	sort.Sort(ByTime(historyCopy))

	historyCopy = historyCopy[from:to]
	return historyCopy, nil
}

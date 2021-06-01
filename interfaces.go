package main

type DB interface {
	Delete(int32) error
	Add(HistoryElement) error
	GetHistory(int, int) ([]*historyCopyElement, error)
}

package main

import (
	"log"
	"time"

	pg "github.com/go-pg/pg/v10"
)

const (
	host     = ":5432"
	user     = "postgres"
	password = "000000"
	dbname   = "RequestDB"
)

type PgDB struct {
	Database *pg.DB
}

type RequestInDB struct {
	tableName struct{} `pg:"requests"`
	Id int
	Method string
	Url string
	Body string
	Time time.Time
	Respstatus int
	Length int
}

func DbPgConnect() (*pg.DB, error) {
	db := pg.Connect(&pg.Options{
		Addr:     host,
		User:     user,
		Password: password,
		Database: dbname,
	})
	return db, nil
}

func NewPGDB() (*PgDB, error) {
	db, err := DbPgConnect()
	if err != nil {
		return nil, err
	}
	return &PgDB{
		Database: db,
	}, nil
}

func (db *PgDB) Add(newHistoryElement HistoryElement) error {

	request := RequestInDB{
		Method: newHistoryElement.Request.Method,
		Url: newHistoryElement.Request.Url,
		Body: newHistoryElement.Request.Body,
		Time: newHistoryElement.Time,
		Respstatus: newHistoryElement.Respond.HttpStatusCode,
		Length: newHistoryElement.Respond.ContentLength,
	}

	_, err := db.Database.Model(&request).Insert()

	if err != nil {
		return err
	}

	return nil
}

func (db *PgDB) Delete(id int32) error {
	request := RequestInDB{}
	 _, err := db.Database.Model(&request).
	 	Where("id = ?", id).
	 	Delete()
	if err != nil {
		return err
	}
	return nil
}

func (db *PgDB) GetHistory(offset, limit int) ([]*historyCopyElement, error) {

	//getting data from Database
	var requests []*RequestInDB

	err := db.Database.Model(&requests).
		Order("time ASC").
		Limit(limit).
		Offset(offset).
		Select()
	if err != nil {
		log.Print(err)
	}

	for _, element := range requests{
		log.Print(element)
	}

	//transforming database struct into target struct
	historyCopy := make([]*historyCopyElement, 0, len(requests))
	for _, req := range requests{
		request := &historyCopyElement{
			ID : int32(req.Id),
			Element : HistoryElement{
				Time : req.Time,
				Request: Request{
					Url: req.Url,
					Method : req.Method,
					Body: req.Body,
				},
				Respond: Respond{
					HttpStatusCode: req.Respstatus,
					ContentLength: req.Length,
				},
			} ,
		}
		historyCopy = append(historyCopy, request)
	}

	//return result
	return historyCopy, nil
}

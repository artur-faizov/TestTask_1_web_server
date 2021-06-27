package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "000000"
	dbname   = "RequestDB"
)

type PgDB struct {
	Database *sql.DB
}

func DbPgConnect() (*sql.DB, error) {
	pgConnection := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db, err := sql.Open("postgres", pgConnection)
	if err != nil {
		return nil, err
	}
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
	//Trying to add new element in DB

	//Creating SQL request that will insert ne element id TestTable - new element is a request we processed
	sqlInsert := `
		INSERT INTO "requests" (method , url, body, time, respstatus, length)
		VALUES ($1, $2, $3, $4,$5, $6)`

	//executing SQl request to add new element, and transfer data as a parameters of request
	_, err := db.Database.Exec(sqlInsert,
		newHistoryElement.Request.Method,
		newHistoryElement.Request.Url,
		newHistoryElement.Request.Body,
		newHistoryElement.Time,
		newHistoryElement.Respond.HttpStatusCode,
		newHistoryElement.Respond.ContentLength,
	)
	if err != nil {
		return err
	}

	return nil
}

func (db *PgDB) Delete(id int32) error {
	//deleting element from Test Table
	sqlStatement1 := `
		DELETE FROM "requests"
		WHERE id = $1;
		`
	_, err := db.Database.Exec(sqlStatement1, id)
	if err != nil {
		return err
	}
	return nil
}

func (db *PgDB) GetHistory(offset, limit int) ([]*historyCopyElement, error) {

	historyCopy := make([]*historyCopyElement, 0)

	if limit == 0 {
		limit = 2147483647
	}
	//we request required data from testTable
	requests, err := db.Database.Query(`
		SELECT * FROM requests 
		ORDER by time ASC
		LIMIT $1 OFFSET $2
	`, limit, offset)

	if err != nil {
		return nil, err
	}

	//We go over result of our request  going line by line
	for requests.Next() {

		elementFromTable := &historyCopyElement{}

		err = requests.Scan(
			&elementFromTable.ID,
			&elementFromTable.Element.Request.Url,
			&elementFromTable.Element.Request.Method,
			&elementFromTable.Element.Request.Body,
			&elementFromTable.Element.Time,
			&elementFromTable.Element.Respond.HttpStatusCode,
			&elementFromTable.Element.Respond.ContentLength,
		)
		if err != nil {
			return historyCopy, err
		}

		//adding each element into whole history in our format
		historyCopy = append(historyCopy, elementFromTable)
	}

	return historyCopy, nil
}

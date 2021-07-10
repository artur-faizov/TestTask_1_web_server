package main

import (
	"database/sql"
	"fmt"
	sqlextention "github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
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

	pgConnection := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db2, err := sqlextention.Connect("postgres", pgConnection)
	if err != nil {
		return nil, err
	}

	if limit == 0 {
		limit = 2147483647
	}

	// first request to count number of elements in table
	numberOfRequests := db2.QueryRowx(`
		Select COUNT(*) from(
			SELECT * FROM requests 
			ORDER by time ASC
			LIMIT $1 OFFSET $2
		)as listofrows
		`, limit, offset)
	var rowsNumber int
	numberOfRequests.Scan(&rowsNumber)

	//Creating slice for final results
	historyCopy := make([]*historyCopyElement, rowsNumber)

	// second request to get data from table
	requests, err := db2.Queryx(`
		SELECT * FROM requests 
		ORDER by time ASC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}

	//We go over result of our request  going line by line
	elementNumberInSlice := 0
	for requests.Next() {

		//elementFromTable := &historyCopyElement{}
		if elementNumberInSlice <= rowsNumber {
			log.Println(elementNumberInSlice)
			historyCopy[elementNumberInSlice] = &historyCopyElement{}
			err = requests.Scan(
				&historyCopy[elementNumberInSlice].ID,
				&historyCopy[elementNumberInSlice].Element.Request.Url,
				&historyCopy[elementNumberInSlice].Element.Request.Method,
				&historyCopy[elementNumberInSlice].Element.Request.Body,
				&historyCopy[elementNumberInSlice].Element.Time,
				&historyCopy[elementNumberInSlice].Element.Respond.HttpStatusCode,
				&historyCopy[elementNumberInSlice].Element.Respond.ContentLength,
			)

			if err != nil {
				return historyCopy, err
			}
			elementNumberInSlice++
		}

	}

	return historyCopy, nil
}

package main

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	_ "github.com/lib/pq"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "000000"
	dbname   = "TestDB"
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
		INSERT INTO "TestTable" (method , url, body, time, "respStatus", length)
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
		DELETE FROM "TestTable"
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
	//we request all data from testTable
	requests, err := db.Database.Query(`SELECT * FROM "TestTable"`)
	if err != nil {
		return nil, err
	}

	//We go over result of our request  going line by line

	for requests.Next() {
		var id int32
		var url string
		var method string
		var body string
		var requestTime time.Time
		var respStatus int
		var length int

		err = requests.Scan(&id, &url, &method, &body, &requestTime, &respStatus, &length)
		if err != nil {
			return historyCopy, err
		}

		//and convert data from each line into historyCopyElement (single element of our report)
		request := Request{
			Method: method,
			Url:    url,
			Body:   body,
		}

		respond := Respond{
			HttpStatusCode: respStatus,
			ContentLength:  length,
		}

		historyElement := HistoryElement{
			Request: request,
			Respond: respond,
			Time:    requestTime,
		}

		historyCopyElement := historyCopyElement{
			ID:      id,
			Element: historyElement,
		}

		//adding each element into whole history in our format
		historyCopy = append(historyCopy, &historyCopyElement)
	}

	//if offset bigger than size of DB we report that request with such offset does not make sense
	if offset > len(historyCopy) {
		return nil, fmt.Errorf("offset %d greater than size of DB %d", offset, len(historyCopy))
	}

	//we convert offset and limit into slice id's that we need to use to select required part of history
	from := offset
	to := len(historyCopy)
	if limit != 0 && limit+offset < to {
		to = offset + limit
	}

	//Sorting all elements in our history by time
	sort.Sort(ByTime(historyCopy))

	//leaving only requested part of history and send it as a result.
	historyCopy = historyCopy[from:to]

	return historyCopy, nil
}

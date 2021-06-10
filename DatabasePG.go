package main

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
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
	LastID   int32
	Database *sql.DB
	mux      *sync.RWMutex
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

func GetLastID(db *sql.DB) (int32, error) {
	sqlRequest := `
		SELECT ID
		from "TestTable"
		ORDER BY ID DESC LIMIT 1
 	`
	var id int
	rows, err := db.Query(sqlRequest)
	for rows.Next() {
		rows.Scan(&id)
	}
	//log.Print(id)

	if err != nil {
		return 0, err
	}
	return int32(id), nil
}

func GetPgDB() (*PgDB, error) {
	db, err := DbPgConnect()
	if err != nil {
		return nil, err
	}
	lastID, err := GetLastID(db)
	if err != nil {
		return nil, err
	}
	return &PgDB{
		LastID:   lastID,
		Database: db,
		mux:      &sync.RWMutex{},
	}, nil
}

func (db *PgDB) Add(newHistoryElement HistoryElement) error {
	log.Print("Trying to add new element in DB")

	id := atomic.AddInt32(&db.LastID, 1)
	db.mux.Lock()
	defer db.mux.Unlock()

	sqlInsert := `
		INSERT INTO "TestTable" (ID, method , url, body, time, "respStatus", length)
		VALUES ($1, $2, $3, $4,$5, $6, $7)`
	_, err := db.Database.Exec(sqlInsert,
		id,
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

	err = addHeaders(db.Database, `"RequestHeaders"`, id, &newHistoryElement.Request.Header)
	if err != nil {
		return err
	}

	//converting Http.Header into map[string][]string
	headers := map[string][]string{}
	for key, values := range newHistoryElement.Respond.Header {
		for _, value := range values {
			headers[key] = append(headers[key], value)
		}
	}

	err = addHeaders(db.Database, `"RespondHeaders"`, id, &headers)
	if err != nil {
		return err
	}

	return nil
}

func addHeaders(db *sql.DB, tableName string, id int32, headers *map[string][]string) error {
	log.Print("recording into Table: ", tableName, " ID: ", id)

	for key, elements := range *headers {
		for _, element := range elements {
			if tableName == `"RequestHeaders"` {
				log.Print("Key: ", key, " value: ", element)
			}
			sqlInsert := fmt.Sprintf(`
				INSERT INTO %s (ID, headername, headervalue)
				VALUES ($1, $2, $3)`, tableName)
			_, err := db.Exec(sqlInsert,
				id,
				key,
				element,
			)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func (db *PgDB) Delete(id int32) error {
	db.mux.Lock()
	defer db.mux.Unlock()

	sqlStatement1 := `
		DELETE FROM "TestTable"
		WHERE id = $1;
		`
	sqlStatement2 := `
		DELETE FROM "RequestHeaders"
		WHERE id = $1;
	`

	sqlStatement3 := `
		DELETE FROM "RespondHeaders"
		WHERE id = $1;
	`
	_, err := db.Database.Exec(sqlStatement1, id)
	if err != nil {
		panic(err)
	}
	_, err = db.Database.Exec(sqlStatement2, id)
	if err != nil {
		panic(err)
	}
	_, err = db.Database.Exec(sqlStatement3, id)
	if err != nil {
		panic(err)
	}
	return nil
}

func (db *PgDB) GetHistory(offset, limit int) ([]*historyCopyElement, error) {

	historyCopy := make([]*historyCopyElement, 0)

	db.mux.RLock()

	requests, err := db.Database.Query(`SELECT * FROM "TestTable"`)
	if err != nil {
		return nil, err
	}

	requestHeaders, err := db.Database.Query(`SELECT * FROM "RequestHeaders"`)
	if err != nil {
		return nil, err
	}

	requestHeadersCopy := make(map[int32]map[string][]string)

	for requestHeaders.Next() {
		var id int32
		var headerName string
		var headerValue string
		requestHeaders.Scan(&id, &headerName, &headerValue)
		if requestHeadersCopy[id] == nil {
			requestHeadersCopy[id] = make(map[string][]string)
		}
		requestHeadersCopy[id][headerName] = append(requestHeadersCopy[id][headerName], headerValue)
	}
	//log.Print(requestHeadersCopy)

	respondHeaders, err := db.Database.Query(`SELECT * FROM "RespondHeaders"`)
	if err != nil {
		return nil, err
	}

	respondHeadersCopy := make(map[int32]map[string][]string)
	for respondHeaders.Next() {
		var id int32
		var headerName string
		var headerValue string
		respondHeaders.Scan(&id, &headerName, &headerValue)
		if respondHeadersCopy[id] == nil {
			respondHeadersCopy[id] = make(map[string][]string)
		}
		respondHeadersCopy[id][headerName] = append(requestHeadersCopy[id][headerName], headerValue)
	}
	//log.Print(respondHeadersCopy)

	for requests.Next() {
		var id int32
		var url string
		var method string
		var body string
		var time time.Time
		var respStatus int
		var length int

		requests.Scan(&id, &url, &method, &body, &time, &respStatus, &length)
		//log.Print(id, " ", requestHeadersCopy[id])
		//log.Print(id, " ", respondHeadersCopy[id])
		request := Request{
			Method: method,
			Url:    url,
			Header: requestHeadersCopy[id],
			Body:   body,
		}

		respond := Respond{
			HttpStatusCode: respStatus,
			ContentLength:  length,
			Header:         respondHeadersCopy[id],
		}

		historyElement := HistoryElement{
			Request: request,
			Respond: respond,
			Time:    time,
		}

		historyCopyElement := historyCopyElement{
			ID:      id,
			Element: historyElement,
		}

		historyCopy = append(historyCopy, &historyCopyElement)

	}

	db.mux.RUnlock()

	if offset > len(historyCopy) {
		return nil, fmt.Errorf("offset %d greater than size of DB %d", offset, len(historyCopy))
	}

	from := offset
	to := len(historyCopy)
	if limit != 0 && limit+offset < to {
		to = offset + limit
	}

	sort.Sort(ByTime(historyCopy))

	historyCopy = historyCopy[from:to]

	return historyCopy, nil
}

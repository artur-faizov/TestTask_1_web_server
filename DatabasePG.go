package main

import (
	"database/sql"
	"fmt"
	"log"
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

func addNewElementToPG(db *sql.DB, newHistoryElement HistoryElement, iteration int) (int32, error) {
	if iteration > 100 {
		return 0, fmt.Errorf("too many attempts to record data into Database, check DB performance. Failed %d attempts", iteration)
	}

	sqlInsert := `
		INSERT INTO "TestTable" (id, method , url, body, time, "respStatus", length)
		VALUES ($1, $2, $3, $4,$5, $6, $7)`

	var id int32

	rows, err := db.Query(`SELECT nextval('"TestTable_id_seq"')`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		err = rows.Scan(&id)
	}

	_, err = db.Exec(sqlInsert,
		id,
		newHistoryElement.Request.Method,
		newHistoryElement.Request.Url,
		newHistoryElement.Request.Body,
		newHistoryElement.Time,
		newHistoryElement.Respond.HttpStatusCode,
		newHistoryElement.Respond.ContentLength,
	)
	if err != nil {
		id, err = addNewElementToPG(db, newHistoryElement, iteration+1)
		if err != nil {
			return 0, err
		}
		return id, nil
	}
	return id, nil
}

func (db *PgDB) Add(newHistoryElement HistoryElement) error {
	//log.Print("Trying to add new element in DB")

	id, err := addNewElementToPG(db.Database, newHistoryElement, 1)
	if err != nil {
		return err
	}
	log.Print("Element added with ID: ", id)

	err = addHeaders(db.Database, `"RequestHeaders"`, id, newHistoryElement.Request.Header)
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

	err = addHeaders(db.Database, `"RespondHeaders"`, id, headers)
	if err != nil {
		return err
	}

	return nil
}

func addHeaders(db *sql.DB, tableName string, id int32, headers map[string][]string) error {
	//log.Print("recording into Table: ", tableName, " ID: ", id)

	for key, elements := range headers {
		for _, element := range elements {
			/*
				if tableName == `"RequestHeaders"` {
					log.Print("Key: ", key, " value: ", element)
				}
			*/
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

	/*
		historyCopy := make([]*historyCopyElement, 0)

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
			err = requestHeaders.Scan(&id, &headerName, &headerValue)
			if err != nil {
				return historyCopy, err
			}
			if requestHeadersCopy[id] == nil {
				requestHeadersCopy[id] = make(map[string][]string)
			}
			requestHeadersCopy[id][headerName] = append(requestHeadersCopy[id][headerName], headerValue)
		}

		for requestHeaders.Next() {
			var id int32
			var headerName string
			var headerValue string
			err = requestHeaders.Scan(&id, &headerName, &headerValue)
			if err != nil {
				return historyCopy, err
			}
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
			err = respondHeaders.Scan(&id, &headerName, &headerValue)
			if err != nil {
				return historyCopy, err
			}
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
			var requestTime time.Time
			var respStatus int
			var length int

			err = requests.Scan(&id, &url, &method, &body, &requestTime, &respStatus, &length)
			if err != nil {
				return historyCopy, err
			}
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
				Time:    requestTime,
			}

			historyCopyElement := historyCopyElement{
				ID:      id,
				Element: historyElement,
			}

			historyCopy = append(historyCopy, &historyCopyElement)
		}
	*/

	historyCopy := make([]*historyCopyElement, 0)

	requests, err := db.Database.Query(`
		SELECT 
			RT.id,
			RT.url,
			RT.method,
			RT.body,
			RT.time,
			RT."respStatus",
			RT.length,
			ReqH.headername as ReqHeaderName,
			ReqH.headervalue as ReqHeaderValue,
			ResH.headername as ResHeaderName,
			ResH.headervalue as ResHeaderValue
		
		FROM "TestTable" RT
				FULL JOIN "RespondHeaders" ResH on RT.id = ResH.id
				FULL JOIN "RequestHeaders" ReqH on RT.id = ReqH.id
	`)
	if err != nil {
		return nil, err
	}

	for requests.Next() {
		var id int32
		var url string
		var method string
		var body string
		var requestTime time.Time
		var respStatus int
		var length int
		var reqHeaderName sql.NullString
		var reqHeaderValue sql.NullString
		var resHeaderName sql.NullString
		var resHeaderValue sql.NullString

		err = requests.Scan(
			&id,
			&url,
			&method,
			&body,
			&requestTime,
			&respStatus,
			&length,
			&reqHeaderName,
			&reqHeaderValue,
			&resHeaderName,
			&resHeaderValue,
		)
		if err != nil {
			return historyCopy, err
		}

		if len(historyCopy) == 0 {

			var reqHeaders = make(map[string][]string)
			if reqHeaderName.String != "" {
				reqHeaders[reqHeaderName.String] = []string{reqHeaderValue.String}
			}

			request := Request{
				Method: method,
				Url:    url,
				Header: reqHeaders,
				Body:   body,
			}

			var resHeaders = make(map[string][]string)
			if resHeaderName.String != "" {
				resHeaders[reqHeaderName.String] = []string{resHeaderValue.String}
			}

			respond := Respond{
				HttpStatusCode: respStatus,
				ContentLength:  length,
				Header:         resHeaders,
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

			historyCopy = append(historyCopy, &historyCopyElement)
			continue
		}

		if historyCopy[len(historyCopy)-1].ID == id {

			RequestHeaderInDB := historyCopy[len(historyCopy)-1].Element.Request.Header

			if reqHeaderName.String != "" {
				val, ok := RequestHeaderInDB[reqHeaderName.String]
				if ok {
					if val[len(val)-1] != reqHeaderValue.String {
						historyCopy[len(historyCopy)-1].Element.Request.Header[reqHeaderName.String] = append(historyCopy[len(historyCopy)-1].Element.Request.Header[reqHeaderName.String], reqHeaderValue.String)
					}
				} else {
					RequestHeaderInDB[reqHeaderName.String] = []string{reqHeaderValue.String}
				}
			}

			RespondHeaderInDB := historyCopy[len(historyCopy)-1].Element.Respond.Header

			if resHeaderName.String != "" {
				val, ok := RespondHeaderInDB[resHeaderName.String]
				if ok {
					if val[len(val)-1] != resHeaderValue.String {
						historyCopy[len(historyCopy)-1].Element.Respond.Header[resHeaderName.String] = append(historyCopy[len(historyCopy)-1].Element.Respond.Header[resHeaderName.String], resHeaderValue.String)
					}
				} else {
					RespondHeaderInDB[resHeaderName.String] = []string{resHeaderValue.String}
				}

			}

		} else {

			var reqHeaders map[string][]string

			if historyCopy[len(historyCopy)-1].Element.Request.Header == nil {
				historyCopy[len(historyCopy)-1].Element.Request.Header = make(map[string][]string)
			}

			RequestHeaderInDB := historyCopy[len(historyCopy)-1].Element.Request.Header

			if reqHeaderName.String != "" {
				val, ok := RequestHeaderInDB[reqHeaderName.String]
				if ok {
					if val[len(val)-1] != reqHeaderValue.String {
						val = append(val, reqHeaderValue.String)
					}
				} else {
					RequestHeaderInDB[reqHeaderName.String] = []string{reqHeaderValue.String}
				}

				reqHeaders = map[string][]string{
					reqHeaderName.String: []string{reqHeaderValue.String},
				}
			} else {
				reqHeaders = map[string][]string{}
			}

			request := Request{
				Method: method,
				Url:    url,
				Header: reqHeaders,
				Body:   body,
			}

			var resHeaders map[string][]string

			if historyCopy[len(historyCopy)-1].Element.Respond.Header == nil {
				historyCopy[len(historyCopy)-1].Element.Respond.Header = make(map[string][]string)
			}

			RespondHeaderInDB := historyCopy[len(historyCopy)-1].Element.Respond.Header
			if resHeaderName.String != "" {
				val, ok := RespondHeaderInDB[resHeaderName.String]
				if ok {
					if val[len(val)-1] != resHeaderValue.String {
						val = append(val, resHeaderValue.String)
					}
				} else {
					RespondHeaderInDB[resHeaderName.String] = []string{resHeaderValue.String}
				}

				resHeaders = map[string][]string{
					resHeaderName.String: []string{resHeaderValue.String},
				}
			} else {
				resHeaders = map[string][]string{}
			}

			respond := Respond{
				HttpStatusCode: respStatus,
				ContentLength:  length,
				Header:         resHeaders,
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

			historyCopy = append(historyCopy, &historyCopyElement)
		}

	}

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

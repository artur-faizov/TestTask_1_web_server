package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

type HistoryElement struct {
	Request Request
	Respond Respond
	Time    time.Time
}

type Request struct {
	Method string              `json:"method"`
	Url    string              `json:"url"`
	Header map[string][]string `json:"header"`
	Body   string              `json:"body"`
}

type Respond struct {
	HttpStatusCode int
	Header         http.Header
	ContentLength  int
}

type historyCopyElement struct {
	ID      int32
	Element HistoryElement
}

func handlers(idb DB) http.Handler {
	r := http.NewServeMux()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "DELETE":
			deleteIdRaw := r.URL.Query().Get("id")
			log.Println("Delete operation requested for ID: ", deleteIdRaw)
			deleteId, err := strconv.ParseInt(deleteIdRaw, 10, 32)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			err = idb.Delete(int32(deleteId))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case "POST":
			reqParams := Request{} //initiate an object to store POST JSON data
			err := json.NewDecoder(r.Body).Decode(&reqParams)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			//Log details about request to do
			log.Printf("Got Request: method: %s url: %s\n", reqParams.Method, reqParams.Url)

			//Executing request to 3rd party service
			var resp *http.Response

			switch reqParams.Method {
			case "GET":
				resp, err = http.Get(reqParams.Url)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

			case "POST":
				//Checking that BODY exist
				if reqParams.Body == "" {
					http.Error(w, "No BODY specified for request", http.StatusBadRequest)
					return
				}
				req, err := http.NewRequest("POST", reqParams.Url, bytes.NewBuffer([]byte(reqParams.Body)))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				for key, element := range reqParams.Header {
					for _, value := range element {
						req.Header.Add(key, value)
					}
				}

				client := &http.Client{}
				resp, err = client.Do(req)
				if err != nil {
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
					return
				}

			default:
				http.Error(w, "HTTP method not in list of supported: GET , POST", http.StatusBadRequest)
				return
			}

			defer func() {
				err := resp.Body.Close()
				if err != nil {
					log.Fatal(err)
				}
			}()

			//Counting ContentLength
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			ContentLength := len(body)

			res := Respond{
				HttpStatusCode: resp.StatusCode,
				Header:         resp.Header,
				ContentLength:  ContentLength,
			}

			historyElement := HistoryElement{
				Request: reqParams,
				Respond: res,
				Time:    time.Now(),
			}

			resJsonNice, err := json.MarshalIndent(res, "", "\t")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {
				_, err = w.Write(resJsonNice)
				if err != nil {
					log.Println(err)
				}
			}

			err = idb.Add(historyElement)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "HTTP method not in list of supported: DELETE , POST", http.StatusBadRequest)
			return
		}

	})

	r.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {

		limit := 0
		offset := 0

		if r.URL.Query()["offset"] != nil {
			ofs, err := strconv.Atoi(r.URL.Query()["offset"][0])
			if err != nil {
				log.Fatalln(err)
			}
			offset = ofs
			log.Println("Offset set to value: ", offset)
		}

		if r.URL.Query()["limit"] != nil {
			l, err := strconv.Atoi(r.URL.Query()["limit"][0])
			if err != nil {
				log.Fatalln(err)
			}
			limit = l
		}

		historyCopy, err := idb.GetHistory(offset, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		jsonHistoryNice, err := json.MarshalIndent(historyCopy, "", "\t")
		if err != nil {
			log.Fatalln(err)
		}

		_, err = w.Write(jsonHistoryNice)
		if err != nil {
			log.Println(err)
		}
	})

	return r
}

func main() {

	//mapDB := NewMapDB()
	pgDB, err := GetPgDB()
	if err != nil {
		panic(err)
	}
	defer pgDB.Database.Close()

	log.Printf("Starting server at port 8080\n")

	if err := http.ListenAndServe(":8080", handlers(pgDB)); err != nil {
		log.Fatal(err)
	}
}

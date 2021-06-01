package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

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

type HistoryElement struct {
	Request Request
	Respond Respond
	Time    time.Time
}

type historyCopyElement struct {
	ID      int32
	Element HistoryElement
}

type ByTime []*historyCopyElement

func (a ByTime) Len() int           { return len(a) }
func (a ByTime) Less(i, j int) bool { return a[i].Element.Time.Before(a[j].Element.Time) }
func (a ByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type myDB struct {
	lastID  *int32
	History map[int32]HistoryElement
	mux     *sync.RWMutex
}

func (db myDB) Add(newHistoryElement HistoryElement) error {

	//log.Print("Current Index value is: ", db.lastID)
	//log.Print("Element is:", newHistoryElement.Request.Url)

	//log.Print("Last ID value before: ", *db.lastID)
	x := atomic.AddInt32(db.lastID, 1)
	//log.Print("Last ID after before: ", *db.lastID)

	//x := len(db.History)
	db.mux.Lock()
	db.History[x] = newHistoryElement
	db.mux.Unlock()
	return nil
}

func (db myDB) Delete(id int32) error {
	db.mux.Lock()
	delete(db.History, id)
	db.mux.Unlock()
	return nil
}

func (db myDB) GetHistory(offset, limit int) ([]*historyCopyElement, error) {

	db.mux.RLock()

	if offset > len(db.History) {
		return nil, fmt.Errorf("offset %d greater than size of DB %d", offset, len(db.History))
	}

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

			/*
				db.mux.Lock()
				delete(db.History, int32(deleteId))
				db.mux.Unlock()
			*/
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
					http.Error(w, "No BODY specified for request", http.StatusBadRequest)
					return
				}

				//x := "{\"method\":\"GET\",\"url\":\"http:\\/\\/mail.ru\"}"

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

			// add request result to History of requests
			/*
				x := atomic.AddInt32(&db.lastID, 1)
				db.mux.Lock()
				db.History[x] = historyElement
				db.mux.Unlock()
			*/
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

		//converting map to slice
		/*
			db.mux.RLock()
			historyCopy := make([]*historyCopyElement, 0)

			for key, element := range db.History {
				historyCopy = append(historyCopy, &historyCopyElement{ID: key, Element: element})
			}
			db.mux.RUnlock()

			sort.Sort(ByTime(historyCopy))
		*/

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

		/*
			if offset > len(historyCopy) {
				offset = len(historyCopy)
			}
		*/

		if r.URL.Query()["limit"] != nil {
			l, err := strconv.Atoi(r.URL.Query()["limit"][0])
			if err != nil {
				log.Fatalln(err)
			}
			limit = l
		}

		/*
			historyCopy = historyCopy[offset:limit]
		*/

		historyCopy, err := idb.GetHistory(offset, limit)

		jsonHistoryNice, err := json.MarshalIndent(historyCopy, "", "\t")
		if err != nil {
			log.Fatalln(err)
		}
		//log.Print(string(jsonHistory))
		//log.Print(string(jsonHistoryNice))

		_, err = w.Write(jsonHistoryNice)
		if err != nil {
			log.Println(err)
		}
	})

	return r
}

func main() {

	id := int32(0)

	idb := &myDB{
		lastID:  &id,
		History: make(map[int32]HistoryElement),
		mux:     &sync.RWMutex{},
	}

	log.Printf("Starting server at port 8080\n")

	if err := http.ListenAndServe(":8080", handlers(idb)); err != nil {
		log.Fatal(err)
	}
}

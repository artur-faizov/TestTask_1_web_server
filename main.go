package main

import (
	"bytes"
	"encoding/json"
	"errors"
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

func rootHandler(r *http.Request, lastID *int32, History map[int32]HistoryElement, mux *sync.RWMutex) (error, int, []byte) {
	switch r.Method {
	case "DELETE":
		deleteIdRaw := r.URL.Query().Get("id")
		log.Println("Delete operation requested for ID: ", deleteIdRaw)
		deleteId, err := strconv.ParseInt(deleteIdRaw, 10, 32)
		if err != nil {
			return err, http.StatusBadRequest, []byte{}
		}

		mux.Lock()
		delete(History, int32(deleteId))
		mux.Unlock()

	case "POST":
		reqParams := Request{} //initiate an object to store POST JSON data
		err := json.NewDecoder(r.Body).Decode(&reqParams)
		if err != nil {
			return err, http.StatusBadRequest, []byte{}
		}

		//Log details about request to do
		log.Printf("Got Request: method: %s url: %s\n", reqParams.Method, reqParams.Url)

		//Executing request to 3rd party service
		var resp *http.Response

		switch reqParams.Method {
		case "GET":
			resp, err = http.Get(reqParams.Url)
			if err != nil {
				return err, http.StatusBadRequest, []byte{}
			}

		case "POST":
			//Checking that BODY exist
			if reqParams.Body == "" {
				return errors.New("no BODY specified for request"), http.StatusBadRequest, []byte{}
			}
			req, err := http.NewRequest("POST", reqParams.Url, bytes.NewBuffer([]byte(reqParams.Body)))
			if err != nil {
				return err, http.StatusInternalServerError, []byte{}
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
				return err, http.StatusServiceUnavailable, []byte{}
			}

		default:
			return errors.New("HTTP method not in list of supported: GET , POST"), http.StatusBadRequest, []byte{}
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
			return err, http.StatusInternalServerError, []byte{}
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

		x := atomic.AddInt32(lastID, 1)
		mux.Lock()
		History[x] = historyElement
		mux.Unlock()

		resJsonNice, err := json.MarshalIndent(res, "", "\t")
		if err != nil {
			return err, http.StatusInternalServerError, []byte{}
		} else {
			return nil, 0, resJsonNice
		}

	default:
		return errors.New("HTTP method not in list of supported: DELETE , POST"), http.StatusBadRequest, []byte{}
	}

	return nil, 0, []byte{}
}

func main() {

	//var lastID int32
	lastID := int32(0)
	History := make(map[int32]HistoryElement)
	mux := &sync.RWMutex{}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err, status, resJsonNice := rootHandler(r, &lastID, History, mux)

		if err != nil {
			http.Error(w, err.Error(), status)
		}

		if len(resJsonNice) > 0 {
			_, err = w.Write(resJsonNice)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
		if err != nil {
			log.Println(err)
		}

	})

	http.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {

		//converting map to slice

		mux.RLock()
		historyCopy := make([]*historyCopyElement, 0)

		for key, element := range History {
			historyCopy = append(historyCopy, &historyCopyElement{ID: key, Element: element})
		}
		mux.RUnlock()

		sort.Sort(ByTime(historyCopy))

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

		if offset > len(historyCopy) {
			offset = len(historyCopy)
		}

		if r.URL.Query()["limit"] != nil {
			l, err := strconv.Atoi(r.URL.Query()["limit"][0])
			if err != nil {
				log.Fatalln(err)
			}
			if l+offset < len(historyCopy) {
				limit = l + offset
			} else {
				limit = len(historyCopy)
			}
		} else {
			limit = len(historyCopy)
		}

		historyCopy = historyCopy[offset:limit]

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

	log.Printf("Starting server at port 8080\n")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Request struct {
	Method string `json:"method"`
	Url    string `json:"url"`
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

func main() {

	var lastID int32

	History := make(map[int32]HistoryElement)

	mux := &sync.RWMutex{}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		switch r.Method {
		case "DELETE":
			deleteIdRaw := r.URL.Query().Get("id")
			log.Println("Delete operation requested for ID: ", deleteIdRaw)
			deleteId, err := strconv.ParseInt(deleteIdRaw, 10, 32)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			mux.Lock()
			delete(History, int32(deleteId))
			mux.Unlock()

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
				defer func() {
					err := resp.Body.Close()
					if err != nil {
						log.Fatal(err)

					}
				}()
				/* Under development POST request
				case "POST":
					resp, err = http.Head(reqParams.Url)
					if err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					defer func() {
						err := resp.Body.Close()
						if err != nil {
							log.Fatal(err)
						}
					}()
				*/
			default:
				http.Error(w, "HTTP method not in list of supported: GET , POST", http.StatusBadRequest)
				return
			}

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
			x := atomic.AddInt32(&lastID, 1)
			mux.Lock()
			History[x] = historyElement
			mux.Unlock()

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

		default:
			http.Error(w, "HTTP method not in list of supported: DELETE , POST", http.StatusBadRequest)
			return
		}

	})

	http.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {

		//converting map to slice
		type historyCopyElement struct {
			ID      int32
			Element HistoryElement
		}
		historyCopy := make([]historyCopyElement, 0)

		mux.Lock()
		for key, element := range History {
			historyCopy = append(historyCopy, historyCopyElement{ID: key, Element: element})
		}
		mux.Unlock()

		for it := 0; it < len(historyCopy); it++ {
			for i := 0; i < (len(historyCopy) - it - 1); i++ {
				if historyCopy[i].Element.Time.After(historyCopy[i+1].Element.Time) {
					historyCopy[i], historyCopy[i+1] = historyCopy[i+1], historyCopy[i]
				}
			}
		}

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
			//log.Println(limit)
		} else {
			limit = len(historyCopy)
		}

		historyCopyRange := make([]historyCopyElement, 0)
		for j := offset; j < limit; j++ {
			historyCopyRange = append(historyCopyRange, historyCopy[j])
		}

		jsonHistoryNice, err := json.MarshalIndent(historyCopyRange, "", "\t")
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

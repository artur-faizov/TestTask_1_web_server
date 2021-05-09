package main

import (
	"bytes"
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
}

type HistoryIndexElement struct {
	ID     int32  //key of element in map storage
	Time   string //time when added in DB
	Status int    // 1: DELETED (removed from history)
}

func main() {

	var lastID int32

	History := make(map[int32]HistoryElement)
	HistoryIndex := make([]HistoryIndexElement, 0)

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

			for i := 0; i < len(HistoryIndex); i++ {
				if HistoryIndex[i].ID == int32(deleteId) {
					//HistoryIndex[i].Status = "DELETED: " + time.Now().Format("2006-01-02 15:04:05.000")
					HistoryIndex[i].Status = 1
					break
				}
			}
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
			}

			// add request result to History of requests
			x := atomic.AddInt32(&lastID, 1)
			mux.Lock()
			History[x] = historyElement
			mux.Unlock()

			// add request to History Index
			mux.Lock()
			HistoryIndex = append(HistoryIndex, HistoryIndexElement{ID: x, Time: time.Now().Format("2006-01-02 15:04:05.000")})
			mux.Unlock()
			//log.Print(HistoryIndex, "\n")
			//log.Println("Element added with ID: ", x)
			//log.Println("Map length: ", len(History))

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
		//?limit=50&offset=0

		//keys, ok := r.URL.Query()["key"]
		//log.Print(r.URL.Query(), "\n")
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

		if offset > len(HistoryIndex) {
			offset = len(HistoryIndex)
		}

		mux.RLock()
		if r.URL.Query()["limit"] != nil {
			l, err := strconv.Atoi(r.URL.Query()["limit"][0])
			if err != nil {
				log.Fatalln(err)
			}
			if l+offset < len(HistoryIndex) {
				limit = l + offset
			} else {
				limit = len(HistoryIndex)
			}
			//log.Println(limit)
		} else {
			limit = len(HistoryIndex)
		}

		type historyRangeElement struct {
			ID      string
			Element HistoryElement
		}
		historyRange := make([]historyRangeElement, 0)

		for i := offset; i < limit; i++ {
			if HistoryIndex[i].Status == 1 {
				historyRange = append(historyRange, historyRangeElement{ID: "DELETED", Element: HistoryElement{}})
			} else {
				historyRange = append(historyRange, historyRangeElement{ID: strconv.Itoa(int(HistoryIndex[i].ID)), Element: History[HistoryIndex[i].ID]})
			}

		}
		mux.RUnlock()

		jsonHistoryNice, err := json.MarshalIndent(historyRange, "", "\t")
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

	http.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {

		mux.RLock()
		jsonHistoryIndex, err := json.MarshalIndent(HistoryIndex, "", "\t")
		if err != nil {
			log.Fatalln(err)
		}
		mux.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err = w.Write(jsonHistoryIndex)
		if err != nil {
			log.Println(err)
		}
	})

	log.Printf("Starting server at port 8080\n")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

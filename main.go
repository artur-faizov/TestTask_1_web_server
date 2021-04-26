package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)


type Request struct {
	Method  string `json:"method"`
	Url  string `json:"url"`
}

type Respond struct {
	HttpStatus    string
	Header        http.Header
	ContentLength int
}

type HistoryElement struct {
	Request Request
	Respond Respond
}

type HistoryIndexElement struct {
	ID int32 //key of element in map storage
	Time string //time when added in DB
	Status string // tag if removed from history
}
/*
func RemoveIndex(s []HistoryIndexElement, index int) []HistoryIndexElement {
	return append(s[:index], s[index+1:]...)
}
 */


func main() {

	var lastID int32

	History := make(map[int32]HistoryElement)
	HistoryIndex := make([]HistoryIndexElement, 0)



	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){

		switch r.Method{
		case "DELETE":
			fmt.Println("Delete operation requested")
			deleteIdString := r.URL.Query().Get("id")
			deleteIdInt64, err := strconv.ParseInt(deleteIdString, 10, 32)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			deleteIdInt32 := int32(deleteIdInt64)
			delete(History, deleteIdInt32)

			// can improve speed here based on "half divide method if use Time of recording as a reference"

			for index, element := range HistoryIndex {
				if element.ID == deleteIdInt32 {
					HistoryIndex[index].Status = "DELETED: " + time.Now().Format("2006-01-02 15:04:05.000")
					//HistoryIndex = RemoveIndex(HistoryIndex, index)
					fmt.Println("removed ID from index array: ", index)
					fmt.Print("Current Index array is: ", HistoryIndex)
					break
				}
			}
		case "POST":
			reqParams := Request{} //initiate an object to store POST JSON data
			err := json.NewDecoder(r.Body).Decode(&reqParams)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			//Log details about request to do
			log.Printf( "Got Request: method: %s url: %s\n", reqParams.Method, reqParams.Url)

			//Executing request to 3rd party service
			var resp *http.Response

			switch reqParams.Method{
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
			case "HEAD":
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
				_,err = w.Write([]byte("unknown request type"))
				if err != nil {
					log.Println(err)
				}
				return
			}

			//Counting ContentLength
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(err)
			}
			ContentLength := len(body)


			res := Respond{
				HttpStatus :   resp.Status,
				Header :       resp.Header,
				ContentLength: ContentLength,
			}

			historyElement := HistoryElement{
				Request : reqParams,
				Respond : res,
			}

			// add request result to History of requests
			x := atomic.AddInt32(&lastID, 1)
			History[x] = historyElement

			// add request to History Index
			var IndexElem HistoryIndexElement
			IndexElem.ID = x
			IndexElem.Time = time.Now().Format("2006-01-02 15:04:05.000")
			HistoryIndex = append(HistoryIndex, IndexElem)
			//fmt.Print(HistoryIndex, "\n")
			//fmt.Println("Element added with ID: ", x)
			//fmt.Println("Map length: ", len(History))

			resJsonNice, err := json.MarshalIndent(res, "", "\t")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			_, err = w.Write(resJsonNice)
			if err != nil {
				log.Println(err)
			}
		}


	})

	http.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request){
		//?limit=50&offset=0

		//keys, ok := r.URL.Query()["key"]
		//fmt.Print(r.URL.Query(), "\n")
		limit := 0
		offset := 0

		if r.URL.Query()["limit"] != nil {
			l, err := strconv.Atoi(r.URL.Query()["limit"][0])
				if err != nil {log.Fatalln(err)}
			if l < len(HistoryIndex){
				limit = l
			} else {
				limit = len(HistoryIndex)
			}
			//fmt.Println(limit)
		} else {
			limit = len(HistoryIndex)
		}

		if r.URL.Query()["offset"] != nil {
			o, err := strconv.Atoi(r.URL.Query()["offset"][0])
				if err != nil {log.Fatalln(err)}
			offset = o
			fmt.Println(offset)
		}

		if  offset > len(HistoryIndex){
			offset = len(HistoryIndex)
		}

		type historyRangeElement struct{
			ID string
			Element HistoryElement
		}
		historyRange := make([]historyRangeElement,0)

		for i:=offset; i< limit; i++{
			if  !strings.Contains(HistoryIndex[i].Status, "DELETED") {
				var element historyRangeElement
				element.ID = strconv.Itoa(int(HistoryIndex[i].ID))
				element.Element = History[HistoryIndex[i].ID]
				historyRange = append(historyRange, element)
			} else {
				var element historyRangeElement
				element.ID = HistoryIndex[i].Status
				historyRange = append(historyRange, element)
			}

		}


		jsonHistoryNice, err := json.MarshalIndent(historyRange, "", "\t")
		if err != nil {
			log.Fatalln(err)
		}
		//fmt.Print(string(jsonHistory))
		//fmt.Print(string(jsonHistoryNice))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_,err = w.Write(jsonHistoryNice)
		if err != nil {
			log.Println(err)
		}
	})

	http.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request){

		jsonHistoryIndex, err := json.MarshalIndent(HistoryIndex, "", "\t")
		if err != nil {
			log.Fatalln(err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_,err = w.Write(jsonHistoryIndex)
		if err != nil {
			log.Println(err)
		}
	})


	fmt.Printf("Starting server at port 8080\n")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}


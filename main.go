package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)


type Request struct {
	Method  string `json:"method"`
	Url  string `json:"url"`
}

type Respond struct {
	HttpStatus  string
	Header http.Header
	RespLen int64
}

type HistoryElement struct {
	Request Request
	Respond Respond
}

type HistoryElementInArray struct {
	ID int32 //key of element in map storage
	Time string //time when added in DB
}

func RemoveIndex(s []HistoryElementInArray, index int) []HistoryElementInArray {
	return append(s[:index], s[index+1:]...)
}


func main() {

	var lastID int32

	History := make(map[int32]HistoryElement)
	History_array := make([]HistoryElementInArray, 0)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){

		fmt.Println(r.Method)
		if r.Method == "DELETE" {
			fmt.Println("Delete operation requested")
			deleteID_string := r.URL.Query().Get("id")
			deleteID_int64, err3 := strconv.ParseInt(deleteID_string, 10, 32)
			if err3 != nil {
				http.Error(w, err3.Error(), http.StatusBadRequest)
				return
			}

			//fmt.Println(deleteID_int32)
			deleteID_int32 := int32(deleteID_int64)
			delete(History, deleteID_int32)

			for index, element := range History_array {
				if element.ID == deleteID_int32{
					History_array = RemoveIndex(History_array, index)
					fmt.Println("removed ID from index array: ", index)
					fmt.Print("Current Index array is: ", History_array)
					break
				}
			}



		} else if r.Method == "POST"{
			reqParams := Request{}
			err1 := json.NewDecoder(r.Body).Decode(&reqParams)
			if err1 != nil {
				http.Error(w, err1.Error(), http.StatusBadRequest)
				return
			}
			//Print details about request to do
			fmt.Printf( "Req Params: %+v\n", reqParams)

			//Json to text
			/*
				reqParamsString, err2 := json.Marshal(reqParams)
				if err2 != nil {
					http.Error(w, err2.Error(), http.StatusBadRequest)
					return
				}

				//w.Write(reqParamsString)
				fmt.Print("Req Params: ", reqParamsString, "\n")
			*/
			resp, err2 := http.Get(reqParams.Url)
			if err2 != nil {
				log.Fatalln(err2)
			}
			defer resp.Body.Close()

			var res Respond
			res.HttpStatus = resp.Status
			res.Header = resp.Header
			res.RespLen = resp.ContentLength
			//fmt.Print("ResLength: ", res.RespLen, "\n")
			// length always -1
			// https://stackoverflow.com/questions/49112440/unexpected-http-net-response-content-length-in-golang
			/*
				for key, element := range res.Header {
					fmt.Println("Header Name: ", key,": ", element)
				}
			*/

			var helement HistoryElement
			helement.Request = reqParams
			helement.Respond = res

			x := atomic.AddInt32(&lastID, 1)
			History[x] = helement

			var his_arr_elem HistoryElementInArray
			his_arr_elem.ID = x
			his_arr_elem.Time = time.Now().Format("2006-01-02 15:04:05.000")
			//fmt.Print(his_arr_elem.Time, "\n")
			History_array = append(History_array, his_arr_elem)
			fmt.Print(History_array, "\n")

			fmt.Println("Element added with ID: ", x)
			fmt.Println("Map length: ", len(History))



			//fmt.Print(res)
			//w.Write()
		}


	})

	http.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request){
		//?limit=50&offset=0
		for key, element := range History {
			fmt.Println("Map key: ", key,": Request: ", element.Request.Method, " ", element.Request.Url)
		}
		/*
		jsonHistory, err := json.Marshal(History)
		if err != nil {
			log.Fatalln(err)
		}

		 */
		jsonHistoryNice, err2 := json.MarshalIndent(History, "", "\t")
		if err2 != nil {
			log.Fatalln(err2)
		}
		//fmt.Print(string(jsonHistory))
		//fmt.Print(string(jsonHistoryNice))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(jsonHistoryNice)
	})



	fmt.Printf("Starting server at port 8080\n")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}


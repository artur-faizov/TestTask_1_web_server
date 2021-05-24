package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

var lastID *int32

func TestMyFirstTest(t *testing.T) {

	myDB_ := myDB{
		LastID:  int32(0),
		History: make(map[int32]HistoryElement),
		mux:     &sync.RWMutex{},
	}

	srv := httptest.NewServer(handlers(&myDB_))
	defer srv.Close()

	bodyContent := "{\"method\": \"GET\", \"url\": \"http://google.com\"}"
	res, err := http.Post(fmt.Sprintf("%s/", srv.URL), "application/json", ioutil.NopCloser(strings.NewReader(bodyContent)))

	if err != nil {
		log.Print(err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("status not OK")
	}
}

package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"
)

var lastID *int32

/*
func TestMain(m *testing.M) {
	runTests := m.Run()
	os.Exit(runTests)
}
*/

func TestMyFirstTest(t *testing.T) {

	r := http.Request{}

	History := make(map[int32]HistoryElement)

	r.Method = "POST"
	bodyContent := "{\"method\": \"GET\", \"url\": \"http://google.com\"}"
	r.Body = ioutil.NopCloser(strings.NewReader(bodyContent))

	lastID := int32(0)
	mux := sync.RWMutex{}
	_, _, p := rootHandler(&r, &lastID, History, &mux)
	log.Print(string(p))

	if len(History) == 0 {
		t.Error("Wrong answer in firs test")
	}

}

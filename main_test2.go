package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockDB struct {
}

func (mdb mockDB) Add(HistoryElement) error {
	return nil
}

func (mdb mockDB) GetHistory(int, int) ([]*historyCopyElement, error) {
	return nil, nil
}

func (mdb mockDB) Delete(int32) error {
	return nil
}

func TestMyFirstTest(t *testing.T) {

	db := mockDB{}

	srv := httptest.NewServer(handlers(db))
	defer srv.Close()

	bodyContent := `{
		"method": "GET", 
		"url": "http://google.com"
	}`
	res, err := http.Post(fmt.Sprintf("%s/", srv.URL), "application/json", ioutil.NopCloser(strings.NewReader(bodyContent)))

	if err != nil {
		log.Print(err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("status not OK")
	}
}

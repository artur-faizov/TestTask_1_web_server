package main

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestDatabase(t *testing.T) {

	id := int32(0)

	testDB := &myDB{
		lastID:  &id,
		History: make(map[int32]HistoryElement),
		mux:     &sync.RWMutex{},
	}

	testRequest := Request{
		Method: `"Get"`,
		Url:    `"http://google.com"`,
		Header: map[string][]string{},
		Body:   `""`,
	}

	testRespond := Respond{
		HttpStatusCode: 100,
		Header:         http.Header{},
		ContentLength:  100,
	}

	testHistoryElement := HistoryElement{
		Request: testRequest,
		Respond: testRespond,
		Time:    time.Now(),
	}

	//Testing Adding new element
	t.Run("Test: Adding element to DB", func(t *testing.T) {
		err := testDB.Add(testHistoryElement)

		if err != nil {
			t.Errorf("error in adding element into DB")
		}

		if len(testDB.History) != 1 {
			t.Errorf("incorrect size of DB:")
		}

		for key, _ := range testDB.History {
			if testDB.History[key].Respond.HttpStatusCode != 100 {
				t.Error("Data corrupted in DB")
			}
		}
	})

	// Testing receiving History  From DB
	t.Run("Test: Receiving History from DB", func(t *testing.T) {
		reportedHistory, err := testDB.GetHistory(0, 0)
		if len(reportedHistory) != 1 {
			t.Error("Wrong number of elements in reported History: ", len(reportedHistory), " Should be: 1")
		}

		// adding test data into db
		testHistoryElement.Request.Url = `"http://google2.com"`
		testHistoryElement.Time = time.Now()
		testDB.Add(testHistoryElement)

		testHistoryElement.Request.Url = `"http://google3.com"`
		testHistoryElement.Time = time.Now()
		testDB.Add(testHistoryElement)

		//testing history order
		reportedHistory, err = testDB.GetHistory(1, 0)
		if reportedHistory[0].Element.Request.Url != `"http://google2.com"` || reportedHistory[1].Element.Request.Url != `"http://google3.com"` {
			t.Error(`Wrong reported History with offset, expected: "http://google2.com" then "http://google3.com" but got: `, reportedHistory[0].Element.Request.Url, " and then:  ", reportedHistory[1].Element.Request.Url)
		}

		//testing offset and limit
		reportedHistory, err = testDB.GetHistory(1, 1)
		if reportedHistory[0].Element.Request.Url != `"http://google2.com"` && len(reportedHistory) != 1 {
			t.Error(`Wrong reported offset and limit, expected: "http://google2.com" but got: `, reportedHistory[0].Element.Request.Url)
		}

		//testing incorrect too big limit
		reportedHistory, err = testDB.GetHistory(2, 100)
		if reportedHistory[0].Element.Request.Url != `"http://google3.com"` && len(reportedHistory) != 1 {
			t.Error(`Error when too big limit `)
		}

		//testing incorrect too big offset
		reportedHistory, err = testDB.GetHistory(100, 100)
		//t.Log(reportedHistory)
		//t.Log(err)
		if err == nil || reportedHistory != nil {
			t.Error("Error when too big offset")
		}
	})

	t.Run("Test: Deletion element from DB", func(t *testing.T) {
		testHistoryElement.Request.Url = "http://DeleteTest.com"
		var testIDForDelete int32
		testDB.Add(testHistoryElement)
		for key, _ := range testDB.History {
			if testDB.History[key].Request.Url == "http://DeleteTest.com" {
				testIDForDelete = key
			}
		}

		if testIDForDelete == 0 {
			t.Error("Error in preparation for deletion test")
		}

		testDB.Delete(testIDForDelete)

		for key, _ := range testDB.History {
			if testDB.History[key].Request.Url == "http://DeleteTest.com" {
				t.Error("Error: element was not removed from DB")
			}
		}

	})

	// Testing deletion of elements
	for key, _ := range testDB.History {
		err := testDB.Delete(key)
		if err != nil {
			t.Error("Error in deleting Database element, ID is:", key)
		}
	}

	if len(testDB.History) != 0 {
		t.Error("Extra data left in DB after erasing all data from it")
	}

}

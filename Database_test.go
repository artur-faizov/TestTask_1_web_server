package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func initTestElement() HistoryElement {
	testRequest := Request{
		Method: `"Get"`,
		Url:    `"http://google.com"`,
		Header: map[string][]string{},
		Body:   `""`,
	}

	testRespond := Respond{
		HttpStatusCode: http.StatusOK,
		Header:         http.Header{},
		ContentLength:  100,
	}

	testHistoryElement := HistoryElement{
		Request: testRequest,
		Respond: testRespond,
		Time:    time.Now(),
	}
	return testHistoryElement
}

func addNTestElements(t *testing.T, testDB *MapDB, testHistoryElement HistoryElement, N int) {
	for i := 0; i < N; i++ {
		err := testDB.Add(testHistoryElement)
		require.Nil(t, err)
	}
	return
}

func TestAdd(t *testing.T) {

	testDB := NewMapDB()
	testHistoryElement := initTestElement()

	//Testing Adding new element
	t.Run("Test: Adding single element to DB", func(t *testing.T) {
		err := testDB.Add(testHistoryElement)
		require.Nil(t, err)
		require.Len(t, testDB.History, 1)

		for key, _ := range testDB.History {
			require.Equal(t, http.StatusOK, testDB.History[key].Respond.HttpStatusCode, "Data is corrupted in DB")
		}
	})
	t.Run("Test: Adding multiple elements to DB", func(t *testing.T) {
		//checking size of DB after adding multiple elements
		N := 101
		addNTestElements(t, testDB, testHistoryElement, N)

		require.Len(t, testDB.History, N+1, "incorrect size of DB with 3 lements. Expected: 3, in facet size is: %d", len(testDB.History))

		for key, _ := range testDB.History {
			require.Condition(t, func() bool {
				return key >= 1 && int(key) <= N+1
			}, "Incorrect ID in DB")
		}
	})
}

func TestDelete(t *testing.T) {

	testDB := NewMapDB()
	testHistoryElement := initTestElement()

	t.Run("Test: Deletion control element from DB", func(t *testing.T) {
		//adding test data
		addNTestElements(t, testDB, testHistoryElement, 100)

		//adding anf delete of control element
		testHistoryElement.Request.Url = "http://DeleteTest.com"
		err := testDB.Add(testHistoryElement)
		require.Nil(t, err, "Error: error in preparation for delete test")

		for key, _ := range testDB.History {
			if testDB.History[key].Request.Url == "http://DeleteTest.com" {
				err := testDB.Delete(key)
				require.Nil(t, err, "Error: error in delete key: %d", key)
			}
		}
		for key, _ := range testDB.History {
			require.NotEqual(t, "http://DeleteTest.com", testDB.History[key].Request.Url, "Error: Target element was not removed from DB")
		}

	})

	// Testing deletion of all elements
	t.Run("Test: Delete all element from DB", func(t *testing.T) {
		for key, _ := range testDB.History {
			err := testDB.Delete(key)
			require.Nil(t, err, "Error in deleting Database element, ID is:", key)
		}
		require.Len(t, testDB.History, 0, "Extra data left in DB after erasing all data from it. Current length is: %d", len(testDB.History))
	})
}

func TestGetHistory(t *testing.T) {

	testDB := NewMapDB()
	testHistoryElement := initTestElement()

	// Testing receiving History  From DB
	t.Run("TestGetHistory: testing basic get history", func(t *testing.T) {

		addNTestElements(t, testDB, testHistoryElement, 100)

		reportedHistory, err := testDB.GetHistory(0, 0)
		require.Nil(t, err)
		require.Len(t, testDB.History, 100, "Wrong number of elements in reported History: ", len(reportedHistory), " Should be: 100")
	})

	t.Run("TestGetHistory: checking order of elements in report", func(t *testing.T) {
		// adding test data into db
		testHistoryElement.Request.Url = "http://google2.com"
		testHistoryElement.Time = time.Now()
		err := testDB.Add(testHistoryElement)
		require.Nil(t, err)

		testHistoryElement.Request.Url = "http://google3.com"
		testHistoryElement.Time = time.Now()
		err = testDB.Add(testHistoryElement)
		require.Nil(t, err)

		//testing history order
		reportedHistory, err := testDB.GetHistory(100, 0)
		require.Nil(t, err, "Error: error in get history")

		require.Len(t, reportedHistory, 2, "Error: wrong length of report")
		require.Equal(t, "http://google2.com", reportedHistory[0].Element.Request.Url, `Wrong reported History with offset, expected: "http://google2.com" but got `, reportedHistory[0].Element.Request.Url)
		require.Equal(t, "http://google3.com", reportedHistory[1].Element.Request.Url, `Wrong reported History with offset, expected: "http://google3.com" but got `, reportedHistory[1].Element.Request.Url)
	})

	t.Run("TestGetHistory: checking offset and limit", func(t *testing.T) {
		//testing offset and limit
		reportedHistory, err := testDB.GetHistory(101, 1)
		if err != nil {
			t.Error("Error: error in get history")
		}
		if len(reportedHistory) != 1 {
			t.Error("Error: wrong length of report")
			return
		}
		if reportedHistory[0].Element.Request.Url != `"http://google2.com"` && len(reportedHistory) != 1 {
			t.Error(`Wrong reported offset and limit, expected: "http://google2.com" but got: `, reportedHistory[0].Element.Request.Url)
		}

		//testing incorrect too big limit
		reportedHistory, err = testDB.GetHistory(101, 1000)
		if err != nil {
			t.Error("Error: error in get history")
		}
		if reportedHistory[0].Element.Request.Url != `"http://google3.com"` && len(reportedHistory) != 1 {
			t.Error(`Error when too big limit `)
		}

		//testing incorrect too big offset
		reportedHistory, err = testDB.GetHistory(200, 100)
		if err == nil || reportedHistory != nil {
			t.Error("Error when too big offset")
		}
	})
}

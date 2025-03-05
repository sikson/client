package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type Root struct {
	XMLName xml.Name `xml:"root"`
	Row     []Item   `xml:"row"`
}
type Item struct {
	Id        string `xml:"id"`
	Guid      string `xml:"guid"`
	Age       int    `xml:"age"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Name      string `xml:"-"`
	About     string `xml:"about"`
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	order_field := r.URL.Query().Get("order_field")
	order_by := r.URL.Query().Get("order_by")
	limit := r.URL.Query().Get("limit")
	offset := r.URL.Query().Get("offset")

	var root Root
	err := root.DecodeXML("dataset.xml")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		fmt.Println("Error:", err)
		return
	}

	root.SearchItems(query)
	err = root.SortRoot(order_field, order_by)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		fmt.Println("Error:", err)
		return
	}

	err = root.ApplyLimitOffset(offset, limit)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		fmt.Println("Error:", err)
		return
	}

	result, _ := json.Marshal(root)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(result)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (r *Root) DecodeXML(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	err = xml.Unmarshal(data, r)
	if err != nil {
		return fmt.Errorf("failed to unmarshal XML: %w", err)
	}
	return nil
}

func (r *Root) SearchItems(query string) {
	var results []Item
	for _, item := range r.Row {
		item.Name = item.FirstName + " " + item.LastName

		if query == "" || strings.Contains(item.Name, query) || strings.Contains(item.About, query) {
			results = append(results, item)
		}
	}
	r.Row = results
}

func (r *Root) SortRoot(orderField string, order string) error {
	if order != "" {
		_, err := strconv.Atoi(order)
		if err != nil {
			return err
		}
	}
	orderInt, _ := strconv.Atoi(order)

	if orderInt != OrderByAsc && orderInt != OrderByAsIs && orderInt != OrderByDesc {
		return fmt.Errorf(ErrorBadOrderField)
	}

	if orderField == "" {
		orderField = "Name"
	}

	switch orderField {
	case "Id":
		sort.Slice(r.Row, func(i, j int) bool {
			if orderInt == OrderByAsc {
				return r.Row[i].Id < r.Row[j].Id
			}
			return r.Row[i].Id > r.Row[j].Id
		})
	case "Age":
		sort.Slice(r.Row, func(i, j int) bool {
			if orderInt == OrderByAsc {
				return r.Row[i].Age < r.Row[j].Age
			}
			return r.Row[i].Age > r.Row[j].Age
		})
	case "Name":
		sort.Slice(r.Row, func(i, j int) bool {
			if orderInt == OrderByAsc {
				return r.Row[i].Name < r.Row[j].Name
			}
			return r.Row[i].Name > r.Row[j].Name
		})
	default:
		return fmt.Errorf(ErrorBadOrderField)
	}
	return nil
}

func (r *Root) ApplyLimitOffset(offset, limit string) error {
	offsetInt := 0
	if offset != "" {
		var err error
		offsetInt, err = strconv.Atoi(offset)
		if err != nil {
			return fmt.Errorf("invalid offset value: %w", err)
		}
	}

	limitInt := len(r.Row) // Если limit пустой, возвращаем все записи
	if limit != "" {
		var err error
		limitInt, err = strconv.Atoi(limit)
		if err != nil {
			return fmt.Errorf("invalid limit value: %w", err)
		}
	}

	if offsetInt >= len(r.Row) {
		r.Row = []Item{}
		return nil
	}

	end := offsetInt + limitInt
	if end > len(r.Row) {
		end = len(r.Row)
	}

	r.Row = r.Row[offsetInt:end]
	return nil
}

type TestCase struct {
	Request          SearchRequest
	ExpectedResponse *SearchResponse
	ExpectError      bool
	ErrorString      string // Ожидаемое сообщение об ошибке
}

func TestSearchClient(t *testing.T) {
	cases := []TestCase{
		{
			Request: SearchRequest{
				Limit:      10,
				Offset:     0,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByAsc,
			},
			ExpectedResponse: &SearchResponse{
				Users: []User{
					{Id: 1, Name: "Alice", Age: 25},
					{Id: 2, Name: "Bob", Age: 30},
				},
				NextPage: false,
			},
			ExpectError: false,
		},
		{
			Request: SearchRequest{
				Limit:      -1,
				Offset:     0,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByAsc,
			},
			ExpectError: true,
			ErrorString: "limit must be > 0",
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	for caseNum, testCase := range cases {
		c := &SearchClient{
			URL: ts.URL,
		}
		resp, err := c.FindUsers(testCase.Request)

		if testCase.ExpectError {
			if err == nil {
				t.Errorf("[%d] expected error, got nil", caseNum)
			} else if err.Error() != testCase.ErrorString {
				t.Errorf("[%d] wrong error, expected %s, got %s", caseNum, testCase.ErrorString, err.Error())
			}
		} else {
			if err != nil {
				t.Errorf("[%d] unexpected error: %#v", caseNum, err)
			}
			if !reflect.DeepEqual(resp, testCase.ExpectedResponse) {
				t.Errorf("[%d] wrong result, expected %#v, got %#v", caseNum, testCase.ExpectedResponse, resp)
			}
		}
	}
	ts.Close()
}

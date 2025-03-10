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
	"time"
)

type Root struct {
	XMLName xml.Name `xml:"root"`
	Row     []Item   `xml:"row"`
}
type Item struct {
	Id        int    `xml:"id"`
	Guid      string `xml:"guid"`
	Age       int    `xml:"age"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Name      string `xml:"-"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

type UserJson struct {
	Id     int    `json:"Id"`
	Name   string `json:"Name"`
	Age    int    `json:"Age"`
	About  string `json:"About"`
	Gender string `json:"Gender"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func JSONError(w http.ResponseWriter, errorMessage interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	errorString := fmt.Sprintf("%v", errorMessage)
	errorResponse := ErrorResponse{Error: errorString}
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	accessToken := r.Header.Get("AccessToken")
	if accessToken == "" {
		JSONError(w, "Bad AccessToken", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("query")
	orderField := r.URL.Query().Get("order_field")
	orderBy := r.URL.Query().Get("order_by")
	limit := r.URL.Query().Get("limit")
	offset := r.URL.Query().Get("offset")

	var root Root
	if err := root.DecodeXML("dataset.xml"); err != nil {
		JSONError(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	root.SearchItems(query)
	if err := root.SortRoot(orderField, orderBy); err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := root.ApplyLimitOffset(offset, limit); err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var users []UserJson
	for _, userXml := range root.Row {
		users = append(users, UserJson{
			Id:     userXml.Id,
			Name:   userXml.Name,
			Age:    userXml.Age,
			About:  userXml.About,
			Gender: userXml.Gender,
		})
	}
	result, _ := json.Marshal(users)
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
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

		if query == "" || strings.Contains(strings.ToLower(item.Name), strings.ToLower(query)) || strings.Contains(strings.ToLower(item.About), strings.ToLower(query)) {
			results = append(results, item)
		}
	}
	r.Row = results
}

func (r *Root) SortRoot(orderField string, order string) error {
	orderInt, err := strconv.Atoi(order)
	if err != nil {
		return err
	}

	if orderInt != OrderByAsc && orderInt != OrderByDesc && orderInt != OrderByAsIs {
		return fmt.Errorf("invalid order: %d", orderInt)
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
		return fmt.Errorf("ErrorBadOrderField")
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

	limitInt := len(r.Row)
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

type TestCaseSearchClient struct {
	Request          SearchRequest
	ExpectedResponse *SearchResponse
}

func TestSearchClient(t *testing.T) {
	cases := []TestCaseSearchClient{
		{
			Request: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "Boyd Wolf",
				OrderField: "Id",
				OrderBy:    -1,
			},
			ExpectedResponse: &SearchResponse{
				Users: []User{
					{
						Id:     0,
						Name:   "Boyd Wolf",
						Age:    22,
						About:  "Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n",
						Gender: "male",
					},
				},
				NextPage: false,
			},
		},
		{
			Request: SearchRequest{
				Limit:      2,
				Offset:     0,
				Query:      "",
				OrderField: "Id",
				OrderBy:    -1,
			},
			ExpectedResponse: &SearchResponse{
				Users: []User{
					{
						Id:     0,
						Name:   "Boyd Wolf",
						Age:    22,
						About:  "Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n",
						Gender: "male",
					},
					{
						Id:     1,
						Name:   "Hilda Mayer",
						Age:    21,
						About:  "Sit commodo consectetur minim amet ex. Elit aute mollit fugiat labore sint ipsum dolor cupidatat qui reprehenderit. Eu nisi in exercitation culpa sint aliqua nulla nulla proident eu. Nisi reprehenderit anim cupidatat dolor incididunt laboris mollit magna commodo ex. Cupidatat sit id aliqua amet nisi et voluptate voluptate commodo ex eiusmod et nulla velit.\n",
						Gender: "female",
					},
				},
				NextPage: true,
			},
		},

		{
			Request: SearchRequest{
				Limit:      26,
				Offset:     0,
				Query:      "Boyd Wolf",
				OrderField: "Id",
				OrderBy:    -1,
			},
			ExpectedResponse: &SearchResponse{
				Users: []User{
					{
						Id:     0,
						Name:   "Boyd Wolf",
						Age:    22,
						About:  "Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n",
						Gender: "male",
					},
				},
				NextPage: false,
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	for caseNum, testCase := range cases {
		c := &SearchClient{
			AccessToken: "123",
			URL:         ts.URL,
		}
		resp, err := c.FindUsers(testCase.Request)
		if err != nil {
			t.Errorf("[%d] unexpected error: %#v", caseNum, err)
		}

		if !reflect.DeepEqual(resp, testCase.ExpectedResponse) {
			t.Errorf("[%d] wrong result, expected %#v, got %#v", caseNum, testCase.ExpectedResponse, resp)
		}
	}
}

type SearchServerErrors struct {
	Request     SearchRequest
	ErrorString string
	IsError     bool
}

func TestSearchServerErrors(t *testing.T) {
	cases := []SearchServerErrors{
		{
			Request: SearchRequest{
				Limit:      -1,
				Offset:     0,
				Query:      "Boyd Wolf",
				OrderField: "Id",
				OrderBy:    -1,
			},
			IsError:     true,
			ErrorString: "limit must be > 0",
		}, {
			Request: SearchRequest{
				Limit:      0,
				Offset:     -1,
				Query:      "Boyd Wolf",
				OrderField: "Id",
				OrderBy:    -1,
			},
			IsError:     true,
			ErrorString: "offset must be > 0",
		}, {
			Request: SearchRequest{
				Limit:      1,
				Offset:     1,
				Query:      "",
				OrderField: "Id",
				OrderBy:    -2,
			},
			IsError:     true,
			ErrorString: "unknown bad request error: invalid order: -2",
		},
		{
			Request: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "",
				OrderField: "About",
				OrderBy:    -1,
			},
			IsError:     true,
			ErrorString: "OrderFeld About invalid",
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	for caseNum, testCase := range cases {
		c := &SearchClient{
			AccessToken: "123",
			URL:         ts.URL,
		}
		_, err := c.FindUsers(testCase.Request)
		if testCase.IsError {
			if err == nil {
				t.Errorf("[%d] expected error, got nil", caseNum)
			} else if err.Error() != testCase.ErrorString {
				t.Errorf("[%d] wrong error, expected %s, got %s", caseNum, testCase.ErrorString, err.Error())
			}
		}
	}
}

func TestFindUsersBadAccessTokenError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	client := &SearchClient{
		URL: ts.URL,
	}
	_, err := client.FindUsers(SearchRequest{
		Limit:  10,
		Offset: 0,
		Query:  "test",
	})
	if err == nil {
		t.Error("Expected error, got nil")
	}
	expectedError := "Bad AccessToken"
	if err.Error() != expectedError {
		t.Errorf("Expected error: %s, got: %s", expectedError, err.Error())
	}
}
func TestFindUsersInternalServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := &SearchClient{
		AccessToken: "valid-token",
		URL:         ts.URL,
	}

	_, err := client.FindUsers(SearchRequest{
		Limit:  10,
		Offset: 0,
		Query:  "test",
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	expectedError := "SearchServer fatal error"
	if err.Error() != expectedError {
		t.Errorf("Expected error: %s, got: %s", expectedError, err.Error())
	}
}

func TestFindUsersJsonError(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		errorResponse := 123
		json.NewEncoder(w).Encode(errorResponse)
	}))
	defer ts.Close()

	client := &SearchClient{
		URL: ts.URL,
	}
	_, err := client.FindUsers(SearchRequest{
		Limit:  10,
		Offset: 0,
		Query:  "test",
	})
	if err == nil {
		t.Error("Expected error, got nil")
	}
	expectedError := "cant unpack error json: json: cannot unmarshal number into Go value of type main.SearchErrorResponse"
	if err.Error() != expectedError {
		t.Errorf("Expected error: %s, got: %s", expectedError, err.Error())
	}
}

func TestFindUsersJsonResultError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(123)
	}))
	defer ts.Close()

	client := &SearchClient{
		URL: ts.URL,
	}
	_, err := client.FindUsers(SearchRequest{
		Limit:  10,
		Offset: 0,
		Query:  "test",
	})
	if err == nil {
		t.Error("Expected error, got nil")
	}
	expectedError := "cant unpack result json: json: cannot unmarshal number into Go value of type []main.User"
	if err.Error() != expectedError {
		t.Errorf("Expected error: %s, got: %s", expectedError, err.Error())
	}
}

func TestFindUsersTimeoutError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
	}))
	defer ts.Close()

	client := &SearchClient{
		AccessToken: "valid-token",
		URL:         ts.URL,
	}

	_, err := client.FindUsers(SearchRequest{
		Limit:  10,
		Offset: 0,
		Query:  "test",
	})

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	expectedError := "timeout for limit=11&offset=0&order_by=0&order_field=&query=test"
	if err.Error() != expectedError {
		t.Errorf("Expected error: %s, got: %s", expectedError, err.Error())
	}
}

func TestFindUsersConnectionError(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close()

	client := &SearchClient{
		AccessToken: "valid-token",
		URL:         ts.URL,
	}

	_, err := client.FindUsers(SearchRequest{
		Limit:  10,
		Offset: 0,
		Query:  "test",
	})

	if err == nil {
		t.Error("Expected connection error, got nil")
	}

	expectedErrorPrefix := "unknown error"
	if !strings.HasPrefix(err.Error(), expectedErrorPrefix) {
		t.Errorf("Expected error to start with: %s, got: %s", expectedErrorPrefix, err.Error())
	}
}

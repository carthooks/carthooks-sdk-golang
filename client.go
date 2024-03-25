package carthooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

type Client struct {
	baseUrl     string
	accessToken string
	httpClient  *http.Client
}

func NewClient(accessToken string) *Client {
	c := &Client{
		accessToken: accessToken,
	}
	if os.Getenv("CARTHOOKS_API_URL") != "" {
		c.baseUrl = os.Getenv("CARTHOOKS_API_URL")
	} else {
		c.baseUrl = "https://api.carthooks.com"
	}
	c.httpClient = &http.Client{}
	return c
}

type Query struct {
	client       *Client
	appID        int
	collectionID int
	limit        int
	filters      map[string]map[string]string
	page         int
	sort         string
}

func (q *Query) Limit(limit int) *Query {
	q.limit = limit
	return q
}

type Item struct {
	ID     int
	Fields map[string]interface{}
}

func (q *Query) Get() ([]Item, error) {
	params := url.Values{}
	if q.limit > 0 {
		params.Add("pagination[pageSize]", strconv.Itoa(int(q.limit)))
	}
	if q.page > 0 {
		params.Add("pagination[page]", strconv.Itoa(int(q.page)))
	}
	if q.sort != "" {
		params.Add("sort", q.sort)
	}
	for field, operators := range q.filters {
		for operator, value := range operators {
			params.Add("filters["+field+"]["+operator+"]", value)
		}
	}
	urladdr := fmt.Sprintf("%s/v1/apps/%d/collections/%d/items?%s",
		q.client.baseUrl, q.appID, q.collectionID, params.Encode())
	fmt.Println(urladdr)
	rst, err := q.client.Get(urladdr)
	if err != nil {
		return nil, err
	}

	items := []Item{}
	err = rst.Bind(&items)
	return items, err
}

type Response struct {
	Data    json.RawMessage        `json:"data"`
	Meta    map[string]interface{} `json:"meta"`
	TraceId string                 `json:"trace_id"`
	Error   *ResponseError         `json:"error"`
}

func (r *Response) Bind(v interface{}) error {
	return json.Unmarshal(r.Data, v)
}

type ResponseError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Key     string `json:"key"`
}

func (c *Client) Get(url string) (*Response, error) {
	return c.Request(http.MethodGet, url, nil)
}

func (c *Client) Post(url string, body map[string]any) (*Response, error) {
	return c.Request(http.MethodPost, url, body)
}

func (c *Client) Request(method, url string, body map[string]any) (*Response, error) {

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	if body != nil {
		jsondata, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req.Body = ioutil.NopCloser(bytes.NewReader(jsondata))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := Response{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, fmt.Errorf("error: %s", result.Error.Key)
	}

	return &result, nil
}

func (q *Query) Filter(field, operator, value string) *Query {
	if q.filters == nil {
		q.filters = make(map[string]map[string]string)
	}
	if q.filters[field] == nil {
		q.filters[field] = make(map[string]string)
	}
	q.filters[field][operator] = value
	return q
}

func (c *Client) Query(appID, collectionID int) *Query {
	return &Query{
		client:       c,
		appID:        appID,
		collectionID: collectionID,
	}
}

func (c *Client) GetItemByID(appID, collectionID, itemID int) (*Item, error) {
	urladdr := fmt.Sprintf("%s/v1/apps/%d/collections/%d/items/%d",
		c.baseUrl, appID, collectionID, itemID)
	rsp, err := c.Get(urladdr)
	if err != nil {
		return nil, err
	}
	item := Item{}
	err = rsp.Bind(&item)
	return &item, err
}

func (c *Client) GetSubmissionToken(appID, collectionID int, options map[string]interface{}) (*Response, error) {
	urladdr := fmt.Sprintf("%s/v1/apps/%d/collections/%d/submission-token",
		c.baseUrl, appID, collectionID)
	return c.Post(urladdr, options)
}

func (c *Client) UpdateSubmissionToken(appID, collectionID, itemID int, options map[string]interface{}) (*Response, error) {
	urladdr := fmt.Sprintf("%s/v1/apps/%d/collections/%d/items/%d/update-token",
		c.baseUrl, appID, collectionID, itemID)
	return c.Post(urladdr, options)
}

func (c *Client) CreateItem(appID, collectionID int, data map[string]interface{}) (item *Item, err error) {
	urladdr := fmt.Sprintf("%s/v1/apps/%d/collections/%d/items",
		c.baseUrl, appID, collectionID)
	rsp, err := c.Post(urladdr, map[string]any{"data": data})
	if err != nil {
		return nil, err
	}
	item = &Item{}
	err = rsp.Bind(item)
	return item, err
}

func (c *Client) UpdateItem(appID, collectionID, itemID int, data map[string]interface{}) (*Response, error) {
	urladdr := fmt.Sprintf("%s/v1/apps/%d/collections/%d/items/%d",
		c.baseUrl, appID, collectionID, itemID)
	return c.Request(http.MethodPut, urladdr, map[string]any{"data": data})
}

func (c *Client) LockItem(appID, collectionID, itemID, lockTimeout int, lockID, subject string) (*Response, error) {
	urladdr := fmt.Sprintf("%s/v1/apps/%d/collections/%d/items/%d/lock",
		c.baseUrl, appID, collectionID, itemID)
	return c.Post(urladdr, map[string]any{
		"lockTimeout": lockTimeout,
		"lockId":      lockID,
		"lockSubject": subject,
	})
}

func (c *Client) UnlockItem(appID, collectionID, itemID int, lockID string) (*Response, error) {
	urladdr := fmt.Sprintf("%s/v1/apps/%d/collections/%d/items/%d/unlock",
		c.baseUrl, appID, collectionID, itemID)
	return c.Post(urladdr, map[string]any{"lockId": lockID})
}

func (c *Client) DeleteItem(appID, collectionID, itemID int) (*Response, error) {
	urladdr := fmt.Sprintf("%s/v1/apps/%d/collections/%d/items/%d",
		c.baseUrl, appID, collectionID, itemID)
	return c.Request(http.MethodDelete, urladdr, nil)
}

func (c *Client) GetUploadToken() (*Response, error) {
	urladdr := fmt.Sprintf("%s/v1/uploads/token", c.baseUrl)
	return c.Post(urladdr, nil)
}

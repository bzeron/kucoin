package kucoin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type CallRequest struct {
	time   time.Time
	url    *url.URL
	method string
	header http.Header
	body   *bytes.Buffer
}

func (c *Client) NewCallRequest(method string, endpoint string, header http.Header, query url.Values, body interface{}) (call *CallRequest, err error) {
	call = &CallRequest{
		time: time.Now(),
		url: &url.URL{
			Scheme:   c.endpoint.Scheme,
			Host:     c.endpoint.Host,
			Path:     endpoint,
			RawQuery: query.Encode(),
		},
		method: method,
		header: header,
		body:   new(bytes.Buffer),
	}
	if header == nil {
		call.header = make(http.Header)
	}
	call.header.Set("User-Agent", fmt.Sprintf("KuCoin-Go-SDK/%s", Version))
	if call.method == http.MethodPost {
		call.header.Set("Content-Type", "application/json")
	}
	if body == nil {
		return
	}
	err = json.NewEncoder(call.body).Encode(body)
	return
}

func (call *CallRequest) request(s *sign) (request *http.Request, err error) {
	request, err = http.NewRequest(call.method, call.url.String(), call.body)
	if err != nil {
		return
	}
	request.Header = call.header
	if s != nil {
		err = s.sign(call)
	}
	return
}

func (call *CallRequest) Pagination(currentPage, pageSize int64) *CallRequest {
	query := call.url.Query()
	query.Set("currentPage", strconv.FormatInt(currentPage, 10))
	query.Set("pageSize", strconv.FormatInt(pageSize, 10))
	call.url.RawQuery = query.Encode()
	return call
}

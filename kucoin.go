package kucoin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	ApiResponseSuccess = "200000"
)

var (
	HttpDefaultClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}
)

type Option func(client *Client)

func WithAuth(key, secret, passphrase string) func(client *Client) {
	s, _ := newSign(key, secret, passphrase)
	return func(client *Client) {
		client.key = key
		client.secret = secret
		client.passphrase = passphrase
		client.sign = s
	}
}

type Client struct {
	key        string
	secret     string
	passphrase string
	sign       *sign
}

func NewClient(options ...Option) (client *Client) {
	client = &Client{}
	for _, option := range options {
		option(client)
	}
	return
}

type callResponse struct {
	Code string          `json:"code"`
	Data json.RawMessage `json:"data"`
}

func (c *Client) Send(call *CallRequest) (buf *bytes.Buffer, err error) {
	var request *http.Request
	request, err = call.request(c.sign)
	if err != nil {
		return
	}
	Logger.Infof("api request method: %s, url: %s, header: %v", request.Method, request.URL.String(), request.Header)
	var response *http.Response
	response, err = HttpDefaultClient.Do(request)
	if err != nil {
		return
	}
	Logger.Infof("api response status code: %d", response.StatusCode)
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("api response http error: [code:%d, message:%s]", response.StatusCode, http.StatusText(response.StatusCode))
		return
	}
	var resp callResponse
	err = json.NewDecoder(response.Body).Decode(&resp)
	if err != nil {
		return
	}
	Logger.Infof("api response code: %s", resp.Code)
	if resp.Code != ApiResponseSuccess {
		err = fmt.Errorf("api response system error: [code:%s, message:%s]", resp.Code, resp.Data)
		return
	}
	buf = bytes.NewBuffer(resp.Data)
	return
}

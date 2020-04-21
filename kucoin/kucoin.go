package kucoin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
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
		Timeout: time.Second * 10,
	}
)

type Option func(client *Client) (err error)

func WithAuth(key, secret, passphrase string) Option {
	return func(client *Client) (err error) {
		client.key = key
		client.secret = secret
		client.passphrase = passphrase
		client.sign = newSign(key, secret, passphrase)
		return
	}
}

func WithEndpoint(endpoint string) Option {
	return func(client *Client) (err error) {
		client.endpoint, err = url.Parse(endpoint)
		return
	}
}

func WithDefaultEndpoint(endpoint string) Option {
	return func(client *Client) (err error) {
		client.endpoint = &url.URL{
			Scheme: "https",
			Host:   "api.kucoin.com",
		}
		return
	}
}

type Client struct {
	endpoint   *url.URL
	key        string
	secret     string
	passphrase string
	sign       *sign
}

func NewClient(options ...Option) (client *Client, err error) {
	client = &Client{}
	for _, option := range options {
		err = option(client)
		if err != nil {
			return
		}
	}
	return
}

type callResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) Send(call *CallRequest) (buf *bytes.Buffer, err error) {
	var request *http.Request
	request, err = call.request(c.sign)
	if err != nil {
		return
	}
	var response *http.Response
	response, err = HttpDefaultClient.Do(request)
	if err != nil {
		return
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("http error: [code:%d, message:%s]", response.StatusCode, http.StatusText(response.StatusCode))
		return
	}
	var resp callResponse
	err = json.NewDecoder(response.Body).Decode(&resp)
	if err != nil {
		return
	}
	if resp.Code != ApiResponseSuccess {
		err = fmt.Errorf("api error: [code:%s, message:%s]", resp.Code, resp.Message)
		return
	}
	buf = bytes.NewBuffer(resp.Data)
	return
}

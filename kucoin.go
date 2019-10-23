package kucoin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
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

func (c *Client) Send(call *callRequest) (b *bytes.Buffer, err error) {
	var request *http.Request
	request, err = call.request(c.sign)
	if err != nil {
		return
	}
	var client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}
	var response *http.Response
	response, err = client.Do(request)
	if err != nil {
		return
	}
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("system error code:%d, message:%s", response.StatusCode, http.StatusText(response.StatusCode))
		return
	}
	var resp callResponse
	err = json.NewDecoder(response.Body).Decode(&resp)
	if err != nil {
		return
	}
	if resp.Code != "200000" {
		err = fmt.Errorf("system error code:%s, message:%s", resp.Code, resp.Data)
		return
	}
	b = bytes.NewBuffer(resp.Data)
	return
}

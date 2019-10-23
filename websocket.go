package kucoin

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type (
	InstanceServer struct {
		PingInterval int64  `json:"pingInterval"`
		Endpoint     string `json:"endpoint"`
		Protocol     string `json:"protocol"`
		Encrypt      bool   `json:"encrypt"`
		PingTimeout  int64  `json:"pingTimeout"`
	}

	Token struct {
		InstanceServers []InstanceServer `json:"instanceServers"`
		Token           string           `json:"token"`
	}
)

func (c *Client) token(endpoint string) (token Token, err error) {
	var call *callRequest
	call, err = NewCallRequest(http.MethodPost, endpoint, nil, nil, nil)
	if err != nil {
		return
	}
	var b *bytes.Buffer
	b, err = c.Send(call)
	if err != nil {
		return
	}
	err = json.NewDecoder(b).Decode(&token)
	return
}

func (c *Client) PublicToken() (token Token, err error) {
	token, err = c.token("/api/v1/bullet-public")
	return
}

func (c *Client) PrivateToken() (token Token, err error) {
	token, err = c.token("/api/v1/bullet-private")
	return
}

func (token Token) WebsocketConnect() (conn *WebsocketConnect, err error) {
	var server = token.InstanceServers[rand.Intn(len(token.InstanceServers))]
	switch server.Protocol {
	case "websocket":
		conn, err = NewConnect(server, token.Token)
	default:
		err = errors.New("error protocol")
	}
	return
}

type (
	WebsocketConnect struct {
		srv InstanceServer

		conn *websocket.Conn

		ctx    context.Context
		cancel context.CancelFunc

		pp    *sync.Map
		ack   *sync.Map
		error chan error
	}

	websocketResponse struct {
		Id       string          `json:"id"`
		Type     string          `json:"type"`
		Data     json.RawMessage `json:"data"`
		Code     int             `json:"code"`
		Topic    string          `json:"topic,omitempty"`
		Subject  string          `json:"subject,omitempty"`
		TunnelId string          `json:"tunnelId,omitempty"`
	}

	websocketRequest struct {
		Id             string `json:"id"`
		Type           string `json:"type"`
		Topic          string `json:"topic,omitempty"`
		NewTunnelId    string `json:"newTunnelId,omitempty"`
		CloseTunnel    string `json:"closeTunnel,omitempty"`
		TunnelId       string `json:"tunnelId,omitempty"`
		PrivateChannel bool   `json:"privateChannel,omitempty"`
		Response       bool   `json:"response,omitempty"`
	}
)

func NewConnect(server InstanceServer, token string) (conn *WebsocketConnect, err error) {
	var uri *url.URL
	uri, err = url.Parse(server.Endpoint)
	if err != nil {
		return
	}
	var query = uri.Query()
	query.Set("token", token)
	uri.RawQuery = query.Encode()

	conn = &WebsocketConnect{
		srv:   server,
		pp:    new(sync.Map),
		ack:   new(sync.Map),
		error: make(chan error, 1),
	}
	var dialer = &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 30 * time.Second,
		ReadBufferSize:   1 << 10,
		WriteBufferSize:  1 << 10,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	conn.conn, _, err = dialer.Dial(uri.String(), nil)
	if err != nil {
		return
	}

	var welcome websocketResponse
	err = conn.conn.ReadJSON(&welcome)
	if err != nil {
		return
	}
	switch welcome.Type {
	case "error":
		err = errors.New(string(welcome.Data))
		return
	case "welcome":
	default:
		err = errors.New("welcome message not received")
		return
	}

	conn.ctx, conn.cancel = context.WithCancel(context.Background())

	go func() {
		err = conn.heartbeat()
		if err != nil {
			conn.error <- err
		}
	}()

	return
}

func (conn *WebsocketConnect) Close() (err error) {
	conn.cancel()
	err = conn.conn.Close()
	return
}

func (conn *WebsocketConnect) write(req *websocketRequest) (err error) {
	var v []byte
	v, err = json.Marshal(req)
	if err != nil {
		return
	}
	err = conn.conn.WriteMessage(websocket.TextMessage, v)
	return
}

func (conn *WebsocketConnect) read(resp *websocketResponse) (err error) {
	var msgT int
	var msg []byte
	msgT, msg, err = conn.conn.ReadMessage()
	if err != nil {
		return
	}
	if msgT != websocket.TextMessage {
		err = errors.New("not support message")
		return
	}
	err = json.Unmarshal(msg, &resp)
	return
}

func (conn *WebsocketConnect) wait(pool *sync.Map, id string) (err error) {
	var wait = make(chan struct{})
	defer close(wait)
	pool.Store(id, wait)
	defer pool.Delete(id)
	select {
	case <-wait:
	case <-time.After(time.Second * time.Duration(5)):
		err = errors.New("wait ack message timeout")
	}
	return
}

func (conn *WebsocketConnect) cancelWait(pool *sync.Map, id string) {
	var wait, ok = pool.Load(id)
	if !ok {
		return
	}
	wait.(chan struct{}) <- struct{}{}
	return
}

func (conn *WebsocketConnect) heartbeat() (err error) {
	var ticker = time.NewTicker(time.Duration(conn.srv.PingInterval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-conn.ctx.Done():
			err = conn.ctx.Err()
			return
		case <-ticker.C:
			var ping = &websocketRequest{
				Id:   strconv.FormatInt(time.Now().UnixNano(), 10),
				Type: "ping",
			}

			err = conn.write(ping)
			if err != nil {
				return
			}
			err = conn.wait(conn.pp, ping.Id)
			return
		}
	}
}

func (conn *WebsocketConnect) Subscribe(topic, tunnelId string, private, ack bool) (err error) {
	var subscribe = &websocketRequest{
		Id:             strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:           "subscribe",
		Topic:          topic,
		TunnelId:       tunnelId,
		PrivateChannel: private,
		Response:       ack,
	}
	err = conn.write(subscribe)
	if err != nil {
		return
	}
	if ack {
		err = conn.wait(conn.ack, subscribe.Id)
	}
	return
}

func (conn *WebsocketConnect) Unsubscribe(topic string, private, ack bool) (err error) {
	var unsubscribe = &websocketRequest{
		Id:             strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:           "unsubscribe",
		Topic:          topic,
		PrivateChannel: private,
		Response:       ack,
	}
	err = conn.write(unsubscribe)
	if err != nil {
		return
	}
	if ack {
		err = conn.wait(conn.ack, unsubscribe.Id)
	}
	return
}

func (conn *WebsocketConnect) OpenTunnel(tunnelId string, ack bool) (err error) {
	var openTunnel = &websocketRequest{
		Id:          strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:        "openTunnel",
		NewTunnelId: tunnelId,
		Response:    ack,
	}
	err = conn.write(openTunnel)
	if err != nil {
		return
	}
	if ack {
		err = conn.wait(conn.ack, openTunnel.Id)
	}
	return
}

func (conn *WebsocketConnect) CloseTunnel(tunnelId string, ack bool) (err error) {
	var closeTunnel = &websocketRequest{
		Id:          strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:        "closeTunnel",
		CloseTunnel: tunnelId,
		Response:    ack,
	}
	err = conn.write(closeTunnel)
	if err != nil {
		return
	}
	if ack {
		err = conn.wait(conn.ack, closeTunnel.Id)
	}
	return
}

func (conn *WebsocketConnect) Listen(handler func(buffer *bytes.Buffer) (err error)) (err error) {
	defer func() { _ = conn.Close() }()
	for {
		select {
		case <-conn.ctx.Done():
			err = conn.ctx.Err()
			return
		case err = <-conn.error:
			return err
		default:
			var resp websocketResponse
			err = conn.read(&resp)
			if err != nil {
				return
			}
			switch resp.Type {
			case "welcome":
				continue
			case "error":
				err = errors.New(string(resp.Data))
				return
			case "pong":
				conn.cancelWait(conn.pp, resp.Id)
			case "ack":
				conn.cancelWait(conn.ack, resp.Id)
			case "message":
				err = handler(bytes.NewBuffer(resp.Data))
				if err != nil {
					return
				}
			default:
				err = errors.New("invalid message type")
				return
			}
		}
	}
}

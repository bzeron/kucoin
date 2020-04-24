package kucoin

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var (
	WebsocketErrorSize = 1
	WebsocketReadSize  = 1 << 8
	WebsocketWriteSize = 1 << 8

	WebsocketAckTimeout = time.Second * time.Duration(5)

	WebsocketDialer = &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 30 * time.Second,
		ReadBufferSize:   1 << 16,
		WriteBufferSize:  1 << 16,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
)

const (
	WebsocketError   = "error"
	WebsocketWelcome = "welcome"
	WebsocketPing    = "ping"
	WebsocketPong    = "pong"
	WebsocketAck     = "ack"
	WebsocketMessage = "message"
	WebsocketNotice  = "notice"
	WebsocketCommand = "command"

	WebsocketMessageSubscribe   = "subscribe"
	WebsocketMessageUnsubscribe = "unsubscribe"
	WebsocketMessageOpenTunnel  = "openTunnel"
	WebsocketMessageCloseTunnel = "closeTunnel"
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
	var call *CallRequest
	call, err = c.NewCallRequest(http.MethodPost, endpoint, nil, nil, nil)
	if err != nil {
		return
	}
	var data *bytes.Buffer
	data, err = c.Send(call)
	if err != nil {
		return
	}
	err = json.NewDecoder(data).Decode(&token)
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

func (token Token) ConnectToInstance() (conn *WebsocketConn, err error) {
	var instance = token.InstanceServers[rand.Intn(len(token.InstanceServers))]
	switch instance.Protocol {
	case "websocket":
		conn, err = NewConnect(instance, token.Token)
	default:
		err = fmt.Errorf("protocol not support")
	}
	return
}

type (
	Event func(data []byte) (err error)

	message struct {
		topic string
		data  []byte
	}

	WebsocketConn struct {
		srv    InstanceServer
		conn   *websocket.Conn
		ctx    context.Context
		cancel context.CancelFunc
		pp     *sync.Map
		ack    *sync.Map
		err    chan error
		r      chan message
		w      chan interface{}
		close  int32
		events *sync.Map
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

func NewConnect(server InstanceServer, token string) (conn *WebsocketConn, err error) {
	var uri *url.URL
	uri, err = url.Parse(server.Endpoint)
	if err != nil {
		return
	}
	var query = uri.Query()
	query.Set("connectId", strconv.FormatInt(time.Now().UnixNano(), 10))
	query.Set("token", token)
	uri.RawQuery = query.Encode()
	conn = &WebsocketConn{
		srv:    server,
		pp:     new(sync.Map),
		ack:    new(sync.Map),
		err:    make(chan error, WebsocketErrorSize),
		r:      make(chan message, WebsocketReadSize),
		w:      make(chan interface{}, WebsocketWriteSize),
		events: new(sync.Map),
	}
	conn.conn, _, err = WebsocketDialer.Dial(uri.String(), nil)
	if err != nil {
		return
	}
	var welcome websocketResponse
	err = conn.conn.ReadJSON(&welcome)
	if err != nil {
		return
	}
	switch welcome.Type {
	case WebsocketError:
		err = fmt.Errorf(string(welcome.Data))
		return
	case WebsocketWelcome:
	default:
		err = fmt.Errorf("websocket not receive welcome message")
		return
	}
	conn.ctx, conn.cancel = context.WithCancel(context.Background())
	go func() {
		err = conn.heartbeat()
		if err != nil {
			conn.err <- err
		}
	}()
	go func() {
		err = conn.write()
		if err != nil {
			conn.err <- err
		}
	}()
	go func() {
		err = conn.read()
		if err != nil {
			conn.err <- err
		}
	}()
	return
}

func (conn *WebsocketConn) Close() (err error) {
	if !conn.Closed() {
		atomic.SwapInt32(&conn.close, 1)
		conn.cancel()
		close(conn.err)
		close(conn.r)
		close(conn.w)
		err = conn.conn.Close()
	}
	return
}

func (conn *WebsocketConn) Closed() (c bool) {
	return atomic.LoadInt32(&conn.close) != 0
}

func (conn *WebsocketConn) wait(pool *sync.Map, id string, t time.Duration) (err error) {
	var wait = make(chan struct{})
	defer close(wait)
	pool.Store(id, wait)
	defer pool.Delete(id)
	select {
	case <-wait:
	case <-time.After(t):
		err = fmt.Errorf("websocket waited ack: %s, timeout: %s", id, t)
	}
	return
}

func (conn *WebsocketConn) cancelWait(pool *sync.Map, id string) {
	var wait, ok = pool.Load(id)
	if !ok {
		return
	}
	wait.(chan struct{}) <- struct{}{}
	return
}

func (conn *WebsocketConn) Subscribe(topic, tunnelId string, private, ack bool, event Event) (err error) {
	conn.events.Store(topic, event)
	var subscribe = &websocketRequest{
		Id:             strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:           WebsocketMessageSubscribe,
		Topic:          topic,
		TunnelId:       tunnelId,
		PrivateChannel: private,
		Response:       ack,
	}
	conn.w <- subscribe
	if ack {
		err = conn.wait(conn.ack, subscribe.Id, WebsocketAckTimeout)
	}
	return
}

func (conn *WebsocketConn) Unsubscribe(topic string, private, ack bool) (err error) {
	var unsubscribe = &websocketRequest{
		Id:             strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:           WebsocketMessageUnsubscribe,
		Topic:          topic,
		PrivateChannel: private,
		Response:       ack,
	}
	conn.w <- unsubscribe
	if ack {
		err = conn.wait(conn.ack, unsubscribe.Id, WebsocketAckTimeout)
	}
	conn.events.Delete(topic)
	return
}

func (conn *WebsocketConn) OpenTunnel(tunnelId string, ack bool) (err error) {
	var openTunnel = &websocketRequest{
		Id:          strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:        WebsocketMessageOpenTunnel,
		NewTunnelId: tunnelId,
		Response:    ack,
	}
	conn.w <- openTunnel
	if ack {
		err = conn.wait(conn.ack, openTunnel.Id, WebsocketAckTimeout)
	}
	return
}

func (conn *WebsocketConn) CloseTunnel(tunnelId string, ack bool) (err error) {
	var closeTunnel = &websocketRequest{
		Id:          strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:        WebsocketMessageCloseTunnel,
		CloseTunnel: tunnelId,
		Response:    ack,
	}
	conn.w <- closeTunnel
	if ack {
		err = conn.wait(conn.ack, closeTunnel.Id, WebsocketAckTimeout)
	}
	return
}

func (conn *WebsocketConn) heartbeat() (err error) {
	var pt = time.NewTicker(time.Millisecond*time.Duration(conn.srv.PingInterval) - time.Second)
	defer pt.Stop()
	var ping = &websocketRequest{
		Id:   strconv.FormatInt(time.Now().UnixNano(), 10),
		Type: WebsocketPing,
	}
	for !conn.Closed() {
		select {
		case <-pt.C:
			ping.Id = strconv.FormatInt(time.Now().UnixNano(), 10)
			conn.w <- ping
			err = conn.wait(conn.pp, ping.Id, time.Duration(conn.srv.PingTimeout)*time.Millisecond)
			if err != nil {
				return
			}
		case <-conn.ctx.Done():
			return
		}
	}
	return
}

func (conn *WebsocketConn) read() (err error) {
	for !conn.Closed() {
		select {
		case <-conn.ctx.Done():
			return
		default:
			var resp websocketResponse
			err = conn.conn.ReadJSON(&resp)
			if err != nil {
				return
			}
			switch resp.Type {
			case WebsocketWelcome:
				continue
			case WebsocketError:
				err = fmt.Errorf(string(resp.Data))
				return
			case WebsocketPong:
				go conn.cancelWait(conn.pp, resp.Id)
			case WebsocketAck:
				go conn.cancelWait(conn.ack, resp.Id)
			case WebsocketMessage, WebsocketCommand, WebsocketNotice:
				conn.r <- message{topic: resp.Topic, data: resp.Data}
			default:
				err = fmt.Errorf("websocket received invalid message")
				return
			}
		}
	}
	return
}

func (conn *WebsocketConn) write() (err error) {
	for !conn.Closed() {
		select {
		case <-conn.ctx.Done():
			return
		case req, ok := <-conn.w:
			if !ok {
				return
			}
			var v []byte
			v, err = json.Marshal(&req)
			if err != nil {
				return
			}
			err = conn.conn.WriteMessage(websocket.TextMessage, v)
			if err != nil {
				return
			}
		}
	}
	return
}

func (conn *WebsocketConn) Listen() (err error) {
	defer func() { _ = conn.Close() }()
	for !conn.Closed() {
		select {
		case <-conn.ctx.Done():
			err = conn.ctx.Err()
		case err = <-conn.err:
		case msg, ok := <-conn.r:
			if !ok {
				return
			}
			event, ok := conn.events.Load(msg.topic)
			if !ok {
				return
			}
			err = event.(Event)(msg.data)
		}
		if err != nil {
			return
		}
	}
	return
}

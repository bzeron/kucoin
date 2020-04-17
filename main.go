package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/bzeron/mk/book"

	"github.com/bzeron/mk/kucoin"
)

var (
	client *kucoin.Client

	clTerm = "\033[2J"
	upTerm = "\033[%dA"
	deTerm = "\033[K"
)

func init() {
	var err error
	client, err = kucoin.NewClient(kucoin.WithEndpoint("https://api.kcs.top"))
	if err != nil {
		panic(err)
	}
}

func snapshot() (l3 *book.BookL3, err error) {
	var query = url.Values{}
	query.Set("symbol", "BTC-USDT")
	var request *kucoin.CallRequest
	request, err = client.NewCallRequest(http.MethodGet, "/api/v1/market/orderbook/level3", nil, query, nil)
	if err != nil {
		return
	}
	var buffer *bytes.Buffer
	buffer, err = client.Send(request)
	if err != nil {
		return
	}
	l3 = book.NewBookL3()
	err = json.NewDecoder(buffer).Decode(&l3)
	return
}

func printBookL2(l2 *book.BookL2) {
	level := 10
	var i = 1
	for ; i <= level*2+1; i++ {
		fmt.Printf(upTerm, i)
		fmt.Printf(deTerm)
	}
	asks, bids := l2.Object(level)
	for _, v := range asks {
		fmt.Println(v)
	}
	fmt.Println("-------------------------------")
	for _, v := range bids {
		fmt.Println(v)
	}
}

func printBookL3(l3 *book.BookL3) {
	level := 10
	var i = 1
	for ; i <= level*2+1; i++ {
		fmt.Printf(upTerm, i)
		fmt.Printf(deTerm)
	}
	asks, bids := l3.Object(level)
	for _, v := range asks {
		fmt.Println(v)
	}
	fmt.Println("-------------------------------")
	for _, v := range bids {
		fmt.Println(v)
	}
}

func printBook(isL2 bool, l3 *book.BookL3) {
	fmt.Printf(clTerm)
	if isL2 {
		for {
			printBookL2(l3.ToL2())
			time.Sleep(time.Second / 10)
		}
	} else {
		for {
			printBookL3(l3)
			time.Sleep(time.Second / 10)
		}
	}
}

type Message struct {
	Sequence     book.Sequence `json:"sequence"`
	Side         string        `json:"side"`
	Type         string        `json:"type"`
	Size         string        `json:"size"`
	OrderId      string        `json:"orderId"`
	Price        string        `json:"price"`
	Time         string        `json:"time"`
	MakerOrderId string        `json:"makerOrderId"`
	NewSize      string        `json:"newSize"`
}

func event(l3 *book.BookL3, msg Message) (err error) {
	l3.SetSequence(msg.Sequence)
	switch msg.Type {
	case "received":
	case "open":
		err = l3.Add(msg.OrderId, msg.Side, msg.Price, msg.Size, msg.Time)
	case "done":
		l3.Del(msg.OrderId)
	case "change":
		err = l3.NewSize(msg.OrderId, msg.NewSize)
	case "match":
		err = l3.SubSize(msg.MakerOrderId, msg.Size)
	}
	return
}

func eventWithBookL3(l3 *book.BookL3) func(conn *kucoin.WebsocketConn, buffer *bytes.Buffer) (err error) {
	return func(conn *kucoin.WebsocketConn, buffer *bytes.Buffer) (err error) {
		var msg Message
		err = json.NewDecoder(buffer).Decode(&msg)
		if err != nil {
			return
		}
		switch {
		case msg.Sequence == l3.GetSequence()+1:
			err = event(l3, msg)
		case msg.Sequence <= l3.GetSequence():
		case msg.Sequence > l3.GetSequence():
			l3, err = snapshot()
		}
		return
	}

}

func main() {
	var isL2 bool
	isL2 = *flag.Bool("l2", false, "l2 default l3")
	flag.Parse()
	token, err := client.PublicToken()
	if err != nil {
		panic(err)
	}
	conn, err := token.ConnectToInstance()
	if err != nil {
		panic(err)
	}
	err = conn.Subscribe("/market/level3:BTC-USDT", "", false, true)
	if err != nil {
		panic(err)
	}
	l3, err := snapshot()
	if err != nil {
		panic(err)
	}
	go printBook(isL2, l3)
	err = conn.Listen(eventWithBookL3(l3))
	if err != nil {
		panic(err)
	}
}

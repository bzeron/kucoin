package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/bzeron/mk/kucoin"
	order_book "github.com/bzeron/mk/order-book"
)

type Message struct {
	Sequence     order_book.Sequence `json:"sequence"`
	Side         string              `json:"side"`
	Type         string              `json:"type"`
	Size         string              `json:"size"`
	OrderId      string              `json:"orderId"`
	Price        string              `json:"price"`
	Time         string              `json:"time"`
	MakerOrderId string              `json:"makerOrderId"`
	NewSize      string              `json:"newSize"`
}

var (
	client = kucoin.NewClient(kucoin.WithEndpoint("https://api.kcs.top"))
)

func snapshot() (book *order_book.BookL3, err error) {
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
	book = order_book.NewBookL3()
	err = json.NewDecoder(buffer).Decode(&book)
	return
}

func printBook(book *order_book.BookL2) {
	level := 10
	var i = 1
	for ; i <= level*2+1; i++ {
		fmt.Printf("\033[%dA", i)
		fmt.Printf("\033[K")
	}
	asks, bids := book.Object(level)
	for _, v := range asks {
		fmt.Println(v)
	}
	fmt.Println("-------------------------------")
	for _, v := range bids {
		fmt.Println(v)
	}
}

func main() {
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
	book, err := snapshot()
	if err != nil {
		panic(err)
	}
	go func() {
		fmt.Print("\033[2J")
		for {
			printBook(book.ToL2())
			time.Sleep(time.Second / 10)
		}
	}()
	err = conn.Listen(func(conn *kucoin.WebsocketConn, buffer *bytes.Buffer) (err error) {
		var msg Message
		err = json.NewDecoder(buffer).Decode(&msg)
		if err != nil {
			return
		}
		switch {
		case msg.Sequence == book.GetSequence()+1:
			book.SetSequence(msg.Sequence)
			switch msg.Type {
			case "received":
			case "open":
				err = book.Add(msg.OrderId, msg.Side, msg.Price, msg.Size, msg.Time)
				if err != nil {
					return
				}
			case "done":
				book.Del(msg.OrderId)
			case "change":
				err = book.NewSize(msg.OrderId, msg.NewSize)
				if err != nil {
					return
				}
			case "match":
				err = book.SubSize(msg.MakerOrderId, msg.Size)
				if err != nil {
					return
				}
			}
		case msg.Sequence <= book.GetSequence():
		case msg.Sequence > book.GetSequence():
			book, err = snapshot()
			if err != nil {
				return
			}
		}
		return
	})
	if err != nil {
		panic(err)
	}
}

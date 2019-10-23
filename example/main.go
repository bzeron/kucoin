package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/bzeron/kucoin"
	"log"
	"os"
)

type (
	Level3Stream struct {
		Sequence     string `json:"sequence"`
		Symbol       string `json:"symbol"`
		Type         string `json:"type"`
		Side         string `json:"side"`
		OrderType    string `json:"orderType"`
		Price        string `json:"price"`
		Funds        string `json:"funds"`
		OrderId      string `json:"orderId"`
		Time         string `json:"time"`
		ClientOid    string `json:"clientOid"`
		Size         string `json:"size"`
		RemainSize   string `json:"remainSize"`
		Reason       string `json:"reason"`
		TakerOrderId string `json:"takerOrderId"`
		MakerOrderId string `json:"makerOrderId"`
		TradeId      string `json:"tradeId"`
		NewSize      string `json:"newSize"`
		OldSize      string `json:"oldSize"`
	}
)

func handler(buffer *bytes.Buffer) (err error) {
	var c Level3Stream
	err = json.NewDecoder(buffer).Decode(&c)
	if err != nil {
		return
	}
	log.Printf("%+v\n", c)
	switch c.Type {
	case "received":
	case "open":
	case "done":
	case "match":
	case "change":
	default:
		err = errors.New("invalid message type")
		return
	}
	return
}

func main() {
	var client = kucoin.NewClient(
		kucoin.WithAuth(os.Getenv("API_KEY"), os.Getenv("API_SECRET"), os.Getenv("API_PASSPHRASE")),
	)
	var err error
	var token kucoin.Token
	token, err = client.PublicToken()
	if err != nil {
		panic(err)
	}

	var websocket *kucoin.WebsocketConnect
	websocket, err = token.WebsocketConnect()
	if err != nil {
		panic(err)
	}

	go func() {
		err = websocket.Subscribe("/market/level3:BTC-USDT", "", false, true)
		if err != nil {
			panic(err)
		}
	}()

	err = websocket.Listen(handler)
	if err != nil {
		panic(err)
	}
}

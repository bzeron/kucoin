package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"time"

	"github.com/bzeron/mk/book"

	"github.com/bzeron/mk/kucoin"
	_ "github.com/joho/godotenv/autoload"
)

var (
	client *kucoin.Client
	symbol = "BTC-USDT"
	clTerm = "\033[2J"
	upTerm = "\033[%dA"
	deTerm = "\033[K"
)

func init() {
	var err error
	client, err = kucoin.NewClient(
		kucoin.WithEndpoint("https://api.kucoin.com"),
		kucoin.WithAuth(os.Getenv("KEY"), os.Getenv("SECRET"), os.Getenv("PASSPHRASE")),
	)
	if err != nil {
		panic(err)
	}
}

func snapshot(l3 *book.L3) (err error) {
	var query = url.Values{}
	query.Set("symbol", symbol)
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
	err = json.NewDecoder(buffer).Decode(&l3)
	return
}

func printBookL2(l2 *book.L2) {
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

func printBookL3(l3 *book.L3) {
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

func printBook(printOut string, l3 *book.L3) {
	switch printOut {
	case "l2":
		fmt.Printf(clTerm)
		for {
			printBookL2(l3.ToL2())
			time.Sleep(time.Second / 100)
		}
	case "l3":
		for {
			printBookL3(l3)
			time.Sleep(time.Second / 100)
		}
	default:
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

func event(l3 *book.L3, msg Message) (err error) {
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

func eventWithBookL3(printOut string) func(data []byte) (err error) {
	l3 := book.NewL3()
	err := snapshot(l3)
	if err != nil {
		panic(err)
	}
	go printBook(printOut, l3)
	return func(data []byte) (err error) {
		var msg Message
		err = json.Unmarshal(data, &msg)
		if err != nil {
			return
		}
		switch {
		case msg.Sequence == l3.GetSequence()+1:
			err = event(l3, msg)
		case msg.Sequence <= l3.GetSequence():
		case msg.Sequence > l3.GetSequence():
			err = snapshot(l3)
		}
		return
	}
}

func pprofServer(enable bool) {
	if !enable {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	err := http.ListenAndServe(":8000", mux)
	if err != nil {
		panic(err)
	}
}

func main() {
	var printOut string
	var enablePprof bool
	flag.StringVar(&printOut, "print", "", "l2 or l3")
	flag.BoolVar(&enablePprof, "pprof", false, "pprof enable")
	flag.Parse()
	go pprofServer(enablePprof)
	token, err := client.PrivateToken()
	if err != nil {
		panic(err)
	}
	conn, err := token.ConnectToInstance()
	if err != nil {
		panic(err)
	}
	err = conn.Subscribe(fmt.Sprintf("/market/level3:%s", symbol), "", false, true, eventWithBookL3(printOut))
	if err != nil {
		panic(err)
	}
	err = conn.Listen()
	if err != nil {
		panic(err)
	}
}

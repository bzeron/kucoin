package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bzeron/mk/kucoin"
	"github.com/bzeron/mk/mesh"
	_ "github.com/joho/godotenv/autoload"
	"github.com/shopspring/decimal"
)

func main() {
	symbol := "KCS-USDT"
	client, err := kucoin.NewClient(
		kucoin.WithEndpoint("https://api.kucoin.com"),
		kucoin.WithAuth(os.Getenv("KEY"), os.Getenv("SECRET"), os.Getenv("PASSPHRASE")),
	)
	if err != nil {
		panic(err)
	}
	token, err := client.PublicToken()
	if err != nil {
		panic(err)
	}
	conn, err := token.ConnectToInstance()
	if err != nil {
		panic(err)
	}
	operate := mesh.NewOrderOperate(client, symbol)
	m := mesh.NewMesh(5, 5, 0.0005, 0.01, operate)
	err = conn.Subscribe(fmt.Sprintf("/market/level3:%s", symbol), "", false, true, func(data []byte) (err error) {
		var msg struct {
			Type    string `json:"type"`
			Size    string `json:"size"`
			OrderId string `json:"orderId"`
			Price   string `json:"price"`
			Time    string `json:"time"`
		}
		err = json.Unmarshal(data, &msg)
		if err != nil {
			return
		}
		switch msg.Type {
		case "received":
		case "open":
		case "done":
		case "change":
		case "match":
			var price decimal.Decimal
			price, err = decimal.NewFromString(msg.Price)
			if err != nil {
				return
			}
			m.Run(price)
		}
		return
	})
	if err != nil {
		panic(err)
	}
	err = conn.Listen()
	if err != nil {
		panic(err)
	}
}

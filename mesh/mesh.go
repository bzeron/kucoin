package mesh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"

	"github.com/bzeron/mk/kucoin"
	uuid "github.com/satori/go.uuid"

	"github.com/shopspring/decimal"
)

type Order struct {
	Id    string
	Side  string
	Price decimal.Decimal
	Size  decimal.Decimal
}

func (order *Order) String() string {
	switch order.Side {
	case "sell":
		return "\033[32m" + order.Side + "\033[0m" + "<" + order.Price.String() + ", " + order.Size.String() + ">"
	case "buy":
		return "\033[31m" + order.Side + " \033[0m" + "<" + order.Price.String() + ", " + order.Size.String() + ">"
	default:
		return ""
	}
}

type Orders []*Order

func (orders Orders) Len() int { return len(orders) }

func (orders Orders) Less(i, j int) bool { return orders[i].Price.GreaterThan(orders[j].Price) }

func (orders Orders) Swap(i, j int) { orders[i], orders[j] = orders[j], orders[i] }

type Mesh struct {
	m           sync.RWMutex
	isInit      bool
	w, h        int64
	unit        float64
	price, size decimal.Decimal
	long        Orders
	short       Orders
	orders      map[int64]*Order
	index       int64
	operate     *OrderOperate
}

func NewMesh(w, h int64, unit, size float64, operate *OrderOperate) (mesh *Mesh) {
	mesh = &Mesh{
		w:       w,
		h:       h,
		unit:    unit,
		size:    decimal.NewFromFloat(size),
		long:    make([]*Order, w),
		short:   make([]*Order, h),
		index:   0,
		operate: operate,
	}
	return mesh
}

func (mesh *Mesh) init(price decimal.Decimal) {
	mesh.orders = map[int64]*Order{0: {Side: "buy", Price: price, Size: mesh.size}}
	var i int64 = 1
	for ; i <= mesh.w; i++ {
		order := &Order{
			Side:  "sell",
			Price: price.Add(decimal.NewFromInt(i).Mul(decimal.NewFromFloat(mesh.unit))).Truncate(5),
			Size:  mesh.size.Truncate(2),
		}
		go mesh.order(order)
		mesh.long[i-1] = order
		mesh.orders[i] = order
	}
	var j int64 = 1
	for ; j <= mesh.h; j++ {
		order := &Order{
			Side:  "buy",
			Price: price.Sub(decimal.NewFromInt(j).Mul(decimal.NewFromFloat(mesh.unit))).Truncate(5),
			Size:  mesh.size.Truncate(2),
		}
		go mesh.order(order)
		mesh.short[j-1] = order
		mesh.orders[-j] = order
	}
	mesh.price = price
	mesh.isInit = true
	return
}

func (mesh *Mesh) order(order *Order) (err error) {
	order.Id, err = mesh.operate.order(order.Side, order.Price, order.Size)
	if err != nil {
		log.Println(err)
	}
	return
}

func (mesh *Mesh) reorder(order *Order) (err error) {
	if order.Id != "" {
		_, err = mesh.operate.cancel(order.Id)
		if err != nil {
			log.Println(err)
			return
		}
	}
	order.Id, err = mesh.operate.order(order.Side, order.Price, order.Size)
	if err != nil {
		log.Println(err)
	}
	return
}

func (mesh *Mesh) Print() {
	mesh.m.RLock()
	defer mesh.m.RUnlock()
	orders := make(Orders, 0, len(mesh.orders))
	for _, v := range mesh.orders {
		orders = append(orders, v)
	}
	sort.Sort(orders)
	for _, v := range orders {
		fmt.Println(v)
	}
}

func (mesh *Mesh) Run(price decimal.Decimal) {
	mesh.m.Lock()
	defer mesh.m.Unlock()
	if !mesh.isInit {
		mesh.init(price)
		return
	}
	index := mesh.index
	order := mesh.orders[index]
	for {
		if index > mesh.w {
			mesh.index = mesh.w
			return
		}
		if index < -mesh.h {
			mesh.index = -mesh.h
			return
		}
		courser := mesh.orders[index]
		switch price.Cmp(order.Price) {
		case -1:
			if price.GreaterThan(courser.Price) {
				mesh.index = index
				return
			}
			if courser.Side != "sell" {
				courser.Side = "sell"
				go mesh.reorder(courser)
			}
			index--
		case 1:
			if price.LessThan(courser.Price) {
				mesh.index = index
				return
			}
			if courser.Side != "buy" {
				courser.Side = "buy"
				go mesh.reorder(courser)
			}
			index++
		default:
			return
		}
	}
}

type OrderOperate struct {
	client *kucoin.Client
	symbol string
}

func NewOrderOperate(client *kucoin.Client, symbol string) *OrderOperate {
	return &OrderOperate{client: client, symbol: symbol}
}

func (operate *OrderOperate) order(side string, price, size decimal.Decimal) (orderId string, err error) {
	var request *kucoin.CallRequest
	request, err = operate.client.NewCallRequest(http.MethodPost, "/api/v1/orders", nil, nil, map[string]interface{}{
		"clientOid": uuid.NewV4().String(),
		"symbol":    operate.symbol,
		"side":      side,
		"price":     price,
		"size":      size,
	})
	if err != nil {
		return
	}
	var buffer *bytes.Buffer
	buffer, err = operate.client.Send(request)
	if err != nil {
		return
	}
	var order struct {
		OrderId string `json:"orderId"`
	}
	err = json.NewDecoder(buffer).Decode(&order)
	if err != nil {
		return
	}
	orderId = order.OrderId
	return
}

func (operate *OrderOperate) cancel(orderId string) (cancelledOrderIds []string, err error) {
	var request *kucoin.CallRequest
	request, err = operate.client.NewCallRequest(http.MethodDelete, fmt.Sprintf("/api/v1/orders/%s", orderId), nil, nil, nil)
	if err != nil {
		return
	}
	var buffer *bytes.Buffer
	buffer, err = operate.client.Send(request)
	if err != nil {
		return
	}
	var order struct {
		CancelledOrderIds []string `json:"cancelledOrderIds"`
	}
	err = json.NewDecoder(buffer).Decode(&order)
	if err != nil {
		return
	}
	cancelledOrderIds = order.CancelledOrderIds
	return
}

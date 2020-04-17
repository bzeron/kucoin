package book

import (
	"encoding/json"
	"strconv"

	"github.com/shopspring/decimal"
)

type OrderL3 struct {
	Id    string
	Side  string
	Price decimal.Decimal
	Size  decimal.Decimal
	Time  int64
}

func NewOrder(id, side, price, size, timestamp string) (order *OrderL3, err error) {
	order = new(OrderL3)
	order.Id = id
	order.Side = side
	order.Price, err = decimal.NewFromString(price)
	if err != nil {
		return
	}
	order.Size, err = decimal.NewFromString(size)
	if err != nil {
		return
	}
	order.Time, err = strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return
	}
	return
}

func (order *OrderL3) MarshalJSON() ([]byte, error) {
	return json.Marshal([4]string{order.Id, order.Price.String(), order.Size.String(), strconv.FormatInt(order.Time, 10)})
}

func (order *OrderL3) String() string {
	return order.Id + "\t" + order.Price.String() + "\t" + order.Size.String() + "\t" + strconv.FormatInt(order.Time, 10)
}

func orderL3AsksCmp(a interface{}, b interface{}) int {
	m := a.(*OrderL3)
	n := b.(*OrderL3)
	if m.Id == n.Id {
		return 0
	}
	switch m.Price.Cmp(n.Price) {
	case -1:
		return -1
	case 0:
		if m.Time < n.Time {
			return -1
		}
		return 1
	case 1:
		return 1
	default:
		return 1
	}
}

func orderL3BidsCmp(a interface{}, b interface{}) int {
	m := a.(*OrderL3)
	n := b.(*OrderL3)
	if m.Id == n.Id {
		return 0
	}
	switch m.Price.Cmp(n.Price) {
	case -1:
		return 1
	case 0:
		if m.Time < n.Time {
			return -1
		}
		return 1
	case 1:
		return -1
	default:
		return 1
	}
}

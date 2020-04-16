package order_book

import (
	"github.com/shopspring/decimal"
)

type OrderL2 struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

func (order *OrderL2) String() string {
	return order.Price.String() + "\t" + order.Size.String()
}

func orderL2AsksCmp(a, b interface{}) int {
	return a.(*OrderL2).Price.Cmp(b.(*OrderL2).Price)
}

func orderL2BidsCmp(a, b interface{}) int {
	return -a.(*OrderL2).Price.Cmp(b.(*OrderL2).Price)
}

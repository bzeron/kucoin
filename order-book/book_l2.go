package order_book

import (
	"time"

	"github.com/shopspring/decimal"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
)

type SideL2 struct {
	tree *rbt.Tree
}

func NewSideL2(tree *rbt.Tree) *SideL2 {
	return &SideL2{tree: tree}
}

func (side *SideL2) math(price decimal.Decimal, math func(decimal.Decimal) decimal.Decimal) {
	order := &OrderL2{Price: price}
	v, found := side.tree.Get(order)
	if found {
		order = v.(*OrderL2)
	}
	order.Size = math(order.Size)
	if order.Size.IsZero() {
		side.tree.Remove(order)
	} else {
		side.tree.Put(order, order)
	}
}

func (side *SideL2) Add(price, newSize decimal.Decimal) {
	side.math(price, func(d decimal.Decimal) decimal.Decimal {
		return d.Add(newSize)
	})
}

func (side *SideL2) Sub(price, newSize decimal.Decimal) {
	side.math(price, func(d decimal.Decimal) decimal.Decimal {
		return d.Sub(newSize)
	})
}

type BookL2 struct {
	Sequence Sequence `json:"sequence"`
	Bids     *SideL2  `json:"bids"`
	Asks     *SideL2  `json:"asks"`
	Time     int64    `json:"time"`
}

func NewBookL2() (book *BookL2) {
	return &BookL2{
		Sequence: 0,
		Asks:     NewSideL2(rbt.NewWith(orderL2AsksCmp)),
		Bids:     NewSideL2(rbt.NewWith(orderL2BidsCmp)),
		Time:     time.Now().UnixNano(),
	}
}

func (book *BookL2) Object(level int) (asks, bids []interface{}) {
	var i, j int
	asks = make([]interface{}, level)
	bids = make([]interface{}, level)
	asksIterator := book.Asks.tree.Iterator()
	for ; asksIterator.Next() && i < level; i++ {
		asks[level-i-1] = asksIterator.Value()
	}
	bidsIterator := book.Bids.tree.Iterator()
	for ; bidsIterator.Next() && j < level; j++ {
		bids[j] = bidsIterator.Value()
	}
	return
}

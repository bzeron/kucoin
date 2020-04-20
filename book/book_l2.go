package book

import (
	"time"

	"github.com/shopspring/decimal"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
)

type sideL2 struct {
	tree *rbt.Tree
}

func newSideL2(tree *rbt.Tree) *sideL2 {
	return &sideL2{tree: tree}
}

func (side *sideL2) math(price decimal.Decimal, math func(oldSize decimal.Decimal) (newSize decimal.Decimal)) {
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

func (side *sideL2) add(price, size decimal.Decimal) {
	side.math(price, func(oldSize decimal.Decimal) (newSize decimal.Decimal) {
		return oldSize.Add(size)
	})
}

func (side *sideL2) sub(price, size decimal.Decimal) {
	side.math(price, func(oldSize decimal.Decimal) (newSize decimal.Decimal) {
		return oldSize.Sub(size)
	})
}

func (side *sideL2) iterator() rbt.Iterator {
	return side.tree.Iterator()
}

type L2 struct {
	Sequence Sequence `json:"sequence"`
	Bids     *sideL2  `json:"bids"`
	Asks     *sideL2  `json:"asks"`
	Time     int64    `json:"time"`
}

func NewL2() (book *L2) {
	return &L2{
		Sequence: 0,
		Asks:     newSideL2(rbt.NewWith(orderL2AsksCmp)),
		Bids:     newSideL2(rbt.NewWith(orderL2BidsCmp)),
		Time:     time.Now().UnixNano(),
	}
}

func (book *L2) Object(level int) (asks, bids []interface{}) {
	var i, j int
	asks = make([]interface{}, level)
	bids = make([]interface{}, level)
	asksIterator := book.Asks.iterator()
	for ; asksIterator.Next() && i < level; i++ {
		asks[level-i-1] = asksIterator.Value()
	}
	bidsIterator := book.Bids.iterator()
	for ; bidsIterator.Next() && j < level; j++ {
		bids[j] = bidsIterator.Value()
	}
	return
}

package order_book

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
)

type SideL3 struct {
	name   string
	tree   *rbt.Tree
	orders map[string]*OrderL3
}

func NewSideL3(name string, tree *rbt.Tree, orders map[string]*OrderL3) *SideL3 {
	return &SideL3{
		name:   name,
		tree:   tree,
		orders: orders,
	}
}

func (side *SideL3) Put(o *OrderL3) {
	side.orders[o.Id] = o
	side.tree.Put(o, o)
}

func (side *SideL3) Del(o *OrderL3) {
	delete(side.orders, o.Id)
	side.tree.Remove(o)
	return
}

func (side *SideL3) Update(o *OrderL3) {
	v, found := side.tree.Get(o)
	if !found {
		return
	}
	item := v.(*OrderL3)
	item.Size = o.Size
	if item.Size.IsZero() {
		side.tree.Remove(item)
	}
}

func (side *SideL3) UnmarshalJSON(b []byte) (err error) {
	var s [][4]string
	err = json.Unmarshal(b, &s)
	if err != nil {
		return
	}
	for _, v := range s {
		var order *OrderL3
		order, err = NewOrder(v[0], side.name, v[1], v[2], v[3])
		if err != nil {
			return
		}
		side.Put(order)
	}
	return
}

type BookL3 struct {
	m      sync.RWMutex
	orders map[string]*OrderL3

	Sequence Sequence `json:"sequence"`
	Bids     *SideL3  `json:"bids"`
	Asks     *SideL3  `json:"asks"`
	Time     int64    `json:"time"`
}

func NewBookL3() (book *BookL3) {
	orders := make(map[string]*OrderL3)
	return &BookL3{
		orders:   orders,
		Sequence: 0,
		Asks:     NewSideL3(Asks, rbt.NewWith(orderL3AsksCmp), orders),
		Bids:     NewSideL3(Bids, rbt.NewWith(orderL3BidsCmp), orders),
		Time:     time.Now().UnixNano(),
	}
}

func (book *BookL3) GetSequence() Sequence {
	book.m.RLock()
	defer book.m.RUnlock()
	return book.Sequence
}

func (book *BookL3) SetSequence(s Sequence) {
	book.m.Lock()
	defer book.m.Unlock()
	book.Sequence = s
}

func (book *BookL3) Add(id, side, price, size, timestamp string) (err error) {
	book.m.Lock()
	defer book.m.Unlock()
	if size == "0" || price == "" {
		return
	}
	var order *OrderL3
	order, err = NewOrder(id, side, price, size, timestamp)
	if err != nil {
		return
	}
	switch order.Side {
	case Bids:
		book.Bids.Put(order)
	case Asks:
		book.Asks.Put(order)
	}
	return
}

func (book *BookL3) Del(orderId string) {
	book.m.Lock()
	defer book.m.Unlock()
	order, found := book.orders[orderId]
	if !found {
		return
	}
	switch order.Side {
	case Bids:
		book.Bids.Del(order)
	case Asks:
		book.Asks.Del(order)
	}
}

func (book *BookL3) Get(orderId string) (order *OrderL3, found bool) {
	book.m.RLock()
	defer book.m.RUnlock()
	order, found = book.orders[orderId]
	return
}

func (book *BookL3) NewSize(orderId, newSize string) (err error) {
	book.m.Lock()
	defer book.m.Unlock()
	var order *OrderL3
	var found bool
	order, found = book.orders[orderId]
	if !found {
		return
	}
	order.Size, err = decimal.NewFromString(newSize)
	if err != nil {
		return
	}
	switch order.Side {
	case Bids:
		if order.Size.IsZero() {
			book.Bids.Del(order)
		} else {
			book.Bids.Update(order)
		}
	case Asks:
		if order.Size.IsZero() {
			book.Asks.Del(order)
		} else {
			book.Asks.Update(order)
		}
	}
	return
}

func (book *BookL3) SubSize(orderId, subSize string) (err error) {
	book.m.Lock()
	defer book.m.Unlock()
	var order *OrderL3
	var found bool
	order, found = book.orders[orderId]
	if !found {
		return
	}
	var size decimal.Decimal
	size, err = decimal.NewFromString(subSize)
	if err != nil {
		return
	}
	order.Size = order.Size.Sub(size)
	switch order.Side {
	case Bids:
		if order.Size.IsZero() {
			book.Bids.Del(order)
		} else {
			book.Bids.Update(order)
		}
	case Asks:
		if order.Size.IsZero() {
			book.Asks.Del(order)
		} else {
			book.Asks.Update(order)
		}
	}
	return
}

func (book *BookL3) Object(level int) (asks, bids []interface{}) {
	book.m.RLock()
	defer book.m.RUnlock()
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

func (book *BookL3) ToL2() (bookL2 *BookL2) {
	book.m.RLock()
	defer book.m.RUnlock()
	bookL2 = NewBookL2()
	bookL2.Sequence = book.Sequence
	bookL2.Time = book.Time
	asksIterator := book.Asks.tree.Iterator()
	for asksIterator.Next() {
		order := asksIterator.Value().(*OrderL3)
		bookL2.Asks.Add(order.Price, order.Size)
	}
	bidsIterator := book.Bids.tree.Iterator()
	for bidsIterator.Next() {
		order := bidsIterator.Value().(*OrderL3)
		bookL2.Bids.Add(order.Price, order.Size)
	}
	return
}

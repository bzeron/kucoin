package book

import (
	"encoding/json"
	"strconv"
)

type Sequence int64

func (s *Sequence) UnmarshalJSON(b []byte) (err error) {
	var x string
	err = json.Unmarshal(b, &x)
	if err != nil {
		return
	}
	var i int64
	i, err = strconv.ParseInt(x, 10, 64)
	*s = Sequence(i)
	return
}

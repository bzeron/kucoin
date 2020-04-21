package kucoin

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
)

type sign struct {
	key        string
	secret     string
	passphrase string
}

func newSign(key, secret, passphrase string) (s *sign) {
	s = &sign{
		key:        key,
		secret:     secret,
		passphrase: passphrase,
	}
	return
}

func (s *sign) sign(call *CallRequest) (err error) {
	call.header.Set("KC-API-KEY", s.key)
	timestamp := strconv.FormatInt(call.time.UnixNano()/1e6, 10)
	temp := new(bytes.Buffer)
	_, err = temp.WriteString(timestamp)
	if err != nil {
		return
	}
	_, err = temp.WriteString(call.method)
	if err != nil {
		return
	}
	_, err = temp.WriteString(call.url.RequestURI())
	if err != nil {
		return
	}
	_, err = temp.Write(call.body.Bytes())
	if err != nil {
		return
	}
	hm := hmac.New(sha256.New, []byte(s.secret))
	_, err = hm.Write(temp.Bytes())
	if err != nil {
		return
	}
	ss := base64.StdEncoding.EncodeToString(hm.Sum(nil))
	call.header.Set("KC-API-SIGN", ss)
	call.header.Set("KC-API-TIMESTAMP", timestamp)
	call.header.Set("KC-API-PASSPHRASE", s.passphrase)
	return
}

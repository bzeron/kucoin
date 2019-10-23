package kucoin

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
)

type sign struct {
	key        *bytes.Buffer
	secret     *bytes.Buffer
	passphrase *bytes.Buffer
}

func newSign(key, secret, passphrase string) (s *sign, err error) {
	s = &sign{
		key:        new(bytes.Buffer),
		secret:     new(bytes.Buffer),
		passphrase: new(bytes.Buffer),
	}
	_, err = s.key.WriteString(key)
	if err != nil {
		return
	}
	_, err = s.secret.WriteString(secret)
	if err != nil {
		return
	}
	_, err = s.passphrase.WriteString(passphrase)
	return
}

func (s *sign) sign(call *callRequest) (err error) {
	call.header.Set("KC-API-KEY", s.key.String())
	temp := new(bytes.Buffer)
	_, err = temp.WriteString(strconv.FormatInt(call.time.UnixNano()/1e6, 10))
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
	hm := hmac.New(sha256.New, s.secret.Bytes())
	_, err = hm.Write(temp.Bytes())
	if err != nil {
		return
	}
	call.header.Set("KC-API-SIGN", base64.StdEncoding.EncodeToString(hm.Sum(nil)))
	call.header.Set("KC-API-TIMESTAMP", strconv.FormatInt(call.time.UnixNano()/1e6, 10))
	call.header.Set("KC-API-PASSPHRASE", s.passphrase.String())
	return
}

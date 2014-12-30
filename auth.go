package main

import (
	"crypto/rsa"
)

type Auth struct {
	Nick string
	Key  *rsa.PublicKey
}

func (a *Auth) Kind() string {
	return "auth"
}

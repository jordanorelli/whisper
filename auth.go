package main

import (
	"crypto/rsa"
)

type AuthRequest struct {
	Nick string
	Key  *rsa.PublicKey
}

func (a *AuthRequest) Kind() string {
	return "auth"
}

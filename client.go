package main

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

type Auth struct {
	Nick string
	Key  rsa.PublicKey
}

func (a *Auth) Kind() string {
	return "auth"
}

func connect() {
	f, err := os.Open(options.key)
	if err != nil {
		exit(1, "unable to open private key file at %s: %v", options.key, err)
	}
	defer f.Close()

	d1 := json.NewDecoder(f)
	var key rsa.PrivateKey
	if err := d1.Decode(&key); err != nil {
		exit(1, "unable to decode key: %v", err)
	}

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", options.host, options.port))
	if err != nil {
		exit(1, "unable to connect to server at %s:%d: %v", options.host, options.port, err)
	}
	auth := Auth{
		Nick: options.nick,
		Key:  key.PublicKey,
	}
	encodeRequest(conn, &auth)

	fmt.Println(conn)
}

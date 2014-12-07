package main

import (
	"code.google.com/p/go.crypto/ssh/terminal"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
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

type ReadWriter struct {
	io.Reader
	io.Writer
}

func connect() {
	if !terminal.IsTerminal(0) {
		exit(1, "yeah you have to run this from a tty")
	}
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
	old, err := terminal.MakeRaw(0)
	if err != nil {
		panic(err)
	}
	defer terminal.Restore(0, old)
	r := &ReadWriter{Reader: os.Stdin, Writer: os.Stdout}
	term := terminal.NewTerminal(r, "> ")

	line, err := term.ReadLine()
	switch err {
	case io.EOF:
		return
	case nil:
		fmt.Println(line)
	default:
		exit(1, "error on line read: %v", err)
	}
}

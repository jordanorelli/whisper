package main

import (
	"code.google.com/p/go.crypto/ssh/terminal"
	"crypto/rsa"
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

type Client struct {
	key  *rsa.PrivateKey
	host string
	port int
	nick string
	conn net.Conn
}

func (c *Client) dial() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.host, c.port))
	if err != nil {
		return fmt.Errorf("client unable to connect: %v", err)
	}
	c.conn = conn
	return nil
}

func (c *Client) handshake() error {
	r := &Auth{Nick: c.nick, Key: c.key.PublicKey}
	return c.sendRequest(r)
}

func (c *Client) sendRequest(r request) error {
	return writeRequest(c.conn, r)
}

func (c *Client) run() {
	if err := c.dial(); err != nil {
		exit(1, "%v", err)
	}
	defer c.conn.Close()
	if err := c.handshake(); err != nil {
		exit(1, "%v", err)
	}
	c.term()
}

func (c *Client) term() {
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

func connect() {
	if !terminal.IsTerminal(0) {
		exit(1, "yeah, this only works from a TTY for now, sry.")
	}

	key, err := privateKey()
	if err != nil {
		exit(1, "unable to open private key file: %v", err)
	}

	client := &Client{
		key:  key,
		host: options.host,
		port: options.port,
		nick: options.nick,
	}
	client.run()
}

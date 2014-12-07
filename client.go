package main

import (
	"bufio"
	"code.google.com/p/go.crypto/ssh/terminal"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"unicode"
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
	key    *rsa.PrivateKey
	host   string
	port   int
	nick   string
	conn   net.Conn
	done   chan interface{}
	mu     sync.Mutex
	prompt string
	line   []rune
	prev   *terminal.State
}

func (c *Client) dial() error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	c.info("dialing %s", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("client unable to connect: %v", err)
	}
	c.info("connected to %s", addr)
	c.conn = conn
	c.prompt = fmt.Sprintf("%s> ", addr)
	return nil
}

func (c *Client) handshake() error {
	r := &Auth{Nick: c.nick, Key: c.key.PublicKey}
	c.info("authenticating as %s", c.nick)
	return c.sendRequest(r)
}

func (c *Client) sendRequest(r request) error {
	return writeRequest(c.conn, r)
}

func (c *Client) info(template string, args ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.trunc()
	fmt.Print("\033[90m# ")
	fmt.Printf(template, args...)
	if !strings.HasSuffix(template, "\n") {
		fmt.Print("\n")
	}
	fmt.Printf("\033[0m")
	c.renderLine()
}

func (c *Client) trunc() {
	fmt.Print("\033[1K") // clear to beginning of the line
	fmt.Print("\r")      // move to beginning of the line
}

func (c *Client) err(template string, args ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.trunc()
	fmt.Print("\033[31m# ")
	fmt.Printf(template, args...)
	if !strings.HasSuffix(template, "\n") {
		fmt.Print("\n")
	}
	fmt.Printf("\033[0m")
	c.renderLine()
}

func (c *Client) run() {
	go c.term()

	if err := c.dial(); err != nil {
		exit(1, "%v", err)
	}
	defer c.conn.Close()
	if err := c.handshake(); err != nil {
		exit(1, "%v", err)
	}
	<-c.done
	if c.prev != nil {
		terminal.Restore(0, c.prev)
	}
}

func (c *Client) renderLine() {
	fmt.Printf("\033[1K")                        // clear to beginning of current line
	fmt.Printf("\r")                             // move to beginning of current line
	fmt.Printf("%s%s", c.prompt, string(c.line)) // print the line with prompt
}

func (c *Client) control(r rune) {
	switch r {
	case 3: // ctrl+c
		c.eof()
	case 4: // EOF
		c.eof()
	case 12: // ctrl+l
		c.clear()
	case 13: // enter
		c.enter()
	case 21: // ctrl+u
		c.clearLine()
	case 127: // backspace
		c.backspace()
	default:
		c.info("undefined control sequence: %v %d %c", r, r, r)
	}
}

func (c *Client) enter() {
	fmt.Print("\n")
	line := string(c.line)
	c.line = make([]rune, 0, 32)
	c.exec(line)
}

func (c *Client) exec(line string) {
	parts := strings.Split(line, " ")
	if len(parts) == 0 {
		c.renderLine()
		return
	}
	switch parts[0] {
	default:
		c.err("unrecognized client command: %s", parts[0])
	}
}

func (c *Client) eof() {
	fmt.Print("\033[1K") // clear to beginning of current line
	fmt.Print("\r")      // move to beginning of current line
	c.done <- 1
}

func (c *Client) clear() {
	fmt.Print("\033[2J")   // clear the screen
	fmt.Print("\033[0;0f") // move to 0, 0
	c.renderLine()
}

func (c *Client) clearLine() {
	c.line = make([]rune, 0, 32)
	c.renderLine()
}

func (c *Client) backspace() {
	if len(c.line) == 0 {
		return
	}
	c.line = c.line[:len(c.line)-1]
	c.renderLine()
}

func (c *Client) term() {
	old, err := terminal.MakeRaw(0)
	if err != nil {
		panic(err)
	}
	c.prev = old
	defer close(c.done)

	in := bufio.NewReader(os.Stdin)
	for {
		r, _, err := in.ReadRune()
		switch err {
		case io.EOF:
			return
		case nil:
		default:
			c.err("error reading rune: %v", err)
		}

		if unicode.IsGraphic(r) {
			c.line = append(c.line, r)
			c.renderLine()
		} else {
			c.control(r)
		}
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
		done: make(chan interface{}),
		line: make([]rune, 0, 32),
	}
	client.run()
}

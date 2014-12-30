package main

import (
	"bufio"
	"code.google.com/p/go.crypto/ssh/terminal"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type ReadWriter struct {
	io.Reader
	io.Writer
}

type Client struct {
	key          *rsa.PrivateKey
	host         string
	port         int
	nick         string
	conn         net.Conn
	done         chan interface{}
	mu           sync.Mutex
	prompt       string
	line         []rune
	prev         *terminal.State
	keyStore     map[string]rsa.PublicKey
	requestCount int
	outstanding  map[int]chan Envelope
}

// establishes a connection to the server
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
	go c.handleMessages()
	return nil
}

// handles messages received from the current server
func (c *Client) handleMessages() {
	messages := make(chan Envelope)
	errors := make(chan error)
	done := make(chan interface{})
	go stream(c.conn, messages, errors, done)
	for {
		select {
		case message := <-messages:
			if err := c.handleMessage(message); err != nil {
				c.err("error handling message from server: %v", err)
			}
		case err := <-errors:
			c.err("server error: %v", err)
		case <-done:
			return
		}
	}
}

// handle a message received from the server
func (c *Client) handleMessage(m Envelope) error {
	c.info("received response for message %d", m.Id)
	res, ok := c.outstanding[m.Id]
	if !ok {
		c.info("%v", m)
		c.err("received message corresponding to no known request id: %d", m.Id)
		return fmt.Errorf("no such id: %d", m.Id)
	}
	res <- m
	close(res)
	return nil
}

func (c *Client) handleNote(raw json.RawMessage) error {
	c.info("unmarshaling note...")
	var enote EncryptedNote
	if err := json.Unmarshal(raw, &enote); err != nil {
		return fmt.Errorf("unable to unmarshal encrypted note: %v", err)
	}

	c.info("aes key ciphertext: %x", enote.Key)
	key, err := rsa.DecryptPKCS1v15(rand.Reader, c.key, enote.Key)
	if err != nil {
		return fmt.Errorf("unable to decrypt aes key from note: %v", err)
	}
	c.info("aes key: %x", key)

	title, err := c.aesDecrypt(key, enote.Title)
	if err != nil {
		return fmt.Errorf("unable to decrypt note title: %v", err)
	}

	body, err := c.aesDecrypt(key, enote.Body)
	if err != nil {
		return fmt.Errorf("unable to decrypt note body: %v", err)
	}

	fmt.Print("\033[37m")
	fmt.Printf("\r%s\n", title)
	fmt.Printf("\033[0m") // unset color choice
	fmt.Printf("%s\n", body)
	return nil
}

func (c *Client) handleListNotes(raw json.RawMessage) error {
	var notes ListNotesResponse
	if err := json.Unmarshal(raw, &notes); err != nil {
		return fmt.Errorf("unable to unmarshal listnotes response: %v", err)
	}

	writeNoteTitle := func(id int, title string) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.trunc()
		fmt.Printf("%d\t%s\n", id, title)
		c.renderLine()
	}

	for _, note := range notes {
		key, err := rsa.DecryptPKCS1v15(rand.Reader, c.key, note.Key)
		if err != nil {
			c.err("unable to decrypt note key: %v", err)
			continue
		}

		title, err := c.aesDecrypt(key, note.Title)
		if err != nil {
			c.err("unable to decrype not title: %v", err)
			continue
		}

		writeNoteTitle(note.Id, string(title))
	}
	return nil
}

func (c *Client) handshake() error {
	r := &Auth{Nick: c.nick, Key: &c.key.PublicKey}
	c.info("authenticating as %s", c.nick)
	promise, err := c.sendRequest(r)
	if err != nil {
		return err
	}
	res := <-promise
	switch res.Kind {
	case "error":
		var e ErrorDoc
		if err := json.Unmarshal(res.Body, &e); err != nil {
			return fmt.Errorf("cannot read server error: %v", err)
		}
		c.err("server error: %v", e.Error())
		close(c.done)
	case "bool":
		c.info(string(res.Body))
	default:
		c.err("i dunno what to do with this")
		close(c.done)
	}
	return err
}

func (c *Client) sendRequest(r request) (chan Envelope, error) {
	e, err := wrapRequest(c.requestCount, r)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	res := make(chan Envelope, 1)
	c.outstanding[c.requestCount] = res
	c.requestCount++
	c.info("sending json request: %s", b)
	if _, err := c.conn.Write(b); err != nil {
		return nil, err
	}
	return res, nil
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
	fmt.Print("\033[31m# ") // set color to red
	fmt.Printf(template, args...)
	if !strings.HasSuffix(template, "\n") {
		fmt.Print("\n")
	}
	fmt.Printf("\033[0m") // unset color choice
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
	case 27: // up
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
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		c.renderLine()
		return
	}
	switch parts[0] {
	case "notes/create":
		c.createNote(parts[1:])
	case "notes/get":
		c.getNote(parts[1:])
	case "notes/list":
		c.listNotes(parts[1:])
	case "keys/get":
		c.fetchKey(parts[1:])
	case "msg/send":
		c.sendMessage(parts[1:])
	case "msg/list":
		c.listMessages(parts[1:])
	case "msg/get":
		c.getMessage(parts[1:])
	default:
		c.err("unrecognized client command: %s", parts[0])
	}
}

// ------------------------------------------------------------------------------
// note functions
// ------------------------------------------------------------------------------

func (c *Client) createNote(args []string) {
	if len(args) < 1 {
		c.err("yeah you need to specify a title.")
		return
	}
	title := strings.Join(args, " ")
	c.info("creating new note: %s", title)
	msg, err := c.readTextBlock()
	if err != nil {
		c.err("%v", err)
		return
	}
	note, err := c.encryptNote(title, msg)
	if err != nil {
		c.err("%v", err)
		return
	}
	if _, err := c.sendRequest(note); err != nil {
		c.err("error sending note: %v", err)
	}
}

func (c *Client) getNote(args []string) {
	if len(args) != 1 {
		c.err("ok notes/get takes exactly 1 argument")
		return
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		c.err("that doesn't look like an int: %v", err)
		return
	}
	res, err := c.sendRequest(GetNoteRequest(id))
	if err != nil {
		c.err("couldn't request note: %v", err)
		return
	}
	e := <-res
	c.handleNote(e.Body)
}

func (c *Client) listNotes(args []string) {
	r := &ListNotes{N: 10}
	res, err := c.sendRequest(r)
	if err != nil {
		c.err("%v", err)
	}
	e := <-res
	c.handleListNotes(e.Body)
}

func (c *Client) encryptNote(title string, message []rune) (*EncryptedNote, error) {
	c.info("encrypting note...")
	note := &Note{
		Title: title,
		Body:  []byte(string(message)),
	}

	c.info("generating random aes key")
	key, err := c.aesKey()
	if err != nil {
		return nil, fmt.Errorf("couldn't encrypt note: failed to make aes key bytes: %v", err)
	}
	c.info("aes key: %x", key)

	ctitle, err := c.aesEncrypt(key, []byte(note.Title))
	if err != nil {
		return nil, fmt.Errorf("couldn't encrypt note: failed to aes encrypt title: %v", err)
	}
	c.info("aes ctitle: %s", ctitle)

	cbody, err := c.aesEncrypt(key, note.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't encrypt note: failed to aes encrypt body: %v", err)
	}
	c.info("aes cbody: %s", cbody)

	ckey, err := rsa.EncryptPKCS1v15(rand.Reader, &c.key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("couldn't encrypt note: failed to rsa encrypt aes key: %v", err)
	}
	c.info("ckey: %x", ckey)

	return &EncryptedNote{
		Key:   ckey,
		Title: ctitle,
		Body:  cbody,
	}, nil
}

// ------------------------------------------------------------------------------
// key functions
// ------------------------------------------------------------------------------

func (c *Client) fetchKey(args []string) {
	if len(args) != 1 {
		c.err("keys/get takes exactly one arg")
		return
	}
	req := KeyRequest(args[0])
	res, err := c.sendRequest(req)
	if err != nil {
		c.err("couldn't send key request: %v", err)
		return
	}
	e := <-res
	c.handleKeyResponse(e.Body)
}

func (c *Client) saveKey(nick string, key rsa.PublicKey) {
	if c.keyStore == nil {
		c.keyStore = make(map[string]rsa.PublicKey, 8)
	}
	c.keyStore[nick] = key
}

func (c *Client) getKey(nick string) (*rsa.PublicKey, error) {
	if key, ok := c.keyStore[nick]; ok {
		return &key, nil
	}
	c.fetchKey([]string{nick})
	if key, ok := c.keyStore[nick]; ok {
		return &key, nil
	}
	return nil, fmt.Errorf("no such key")
}

func (c *Client) handleKeyResponse(body json.RawMessage) error {
	// c.info(string(body))
	var res KeyResponse
	if err := json.Unmarshal(body, &res); err != nil {
		c.err(err.Error())
		return err
	}
	// c.info("%v", res)
	c.saveKey(res.Nick, res.Key)
	return nil
}

// ------------------------------------------------------------------------------
// message functions
// ------------------------------------------------------------------------------

func (c *Client) sendMessage(args []string) {
	if len(args) != 1 {
		c.err("send message requires exactly 1 arg, saw %d", len(args))
		return
	}
	to := args[0]

	c.info("fetching key...")
	pkey, err := c.getKey(to)
	if err != nil {
		c.err("%v", err)
		return
	}
	c.info("ok we have a key")

	text, err := c.readTextBlock()
	if err != nil {
		c.err("%v", err)
		return
	}

	aesKey, err := c.aesKey()
	if err != nil {
		c.err("couldn't create an aes key: %v", err)
		return
	}

	ctext, err := c.aesEncrypt(aesKey, []byte(string(text)))
	if err != nil {
		c.err("couldn't aes encrypt message text: %v", err)
		return
	}

	cnick, err := c.aesEncrypt(aesKey, []byte(c.nick))
	if err != nil {
		c.err("couldn't aes encrypt nick: %v", err)
		return
	}

	ckey, err := rsa.EncryptPKCS1v15(rand.Reader, pkey, aesKey)
	if err != nil {
		c.err("couldn't rsa encrypt aes key: %v", err)
		return
	}

	m := Message{
		Key:  ckey,
		From: cnick,
		To:   to,
		Text: ctext,
	}

	res, err := c.sendRequest(m)
	if err != nil {
		c.err("%v", err)
		return
	}
	c.info("%v", <-res)
}

func (c *Client) listMessages(args []string) {
	r := &ListMessages{N: 10}
	promise, err := c.sendRequest(r)
	if err != nil {
		c.err("%v", err)
	}
	env := <-promise

	writeMessageId := func(id int, from string) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.trunc()
		fmt.Printf("%d\t%s\n", id, from)
		c.renderLine()
	}

	var res ListMessagesResponse
	if err := json.Unmarshal(env.Body, &res); err != nil {
		c.err("couldn't read list messages response: %v", err)
		return
	}
	for _, item := range res {
		key, err := c.rsaDecrypt(item.Key)
		if err != nil {
			c.err("unable to read aes key: %v", err)
			return
		}
		from, err := c.aesDecrypt(key, item.From)
		if err != nil {
			c.err("unable to read message sender: %v", err)
			return
		}
		writeMessageId(item.Id, string(from))
	}
}

func (c *Client) getMessage(args []string) {
	if len(args) != 1 {
		c.err("msg/get requires exactly 1 argument: the id of the message to get")
		return
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		c.err("%v", err)
		return
	}

	promise, err := c.sendRequest(GetMessage{Id: id})
	if err != nil {
		c.err("%v", err)
		return
	}

	raw := <-promise

	var msg Message
	if err := json.Unmarshal(raw.Body, &msg); err != nil {
		c.err("%v", err)
		return
	}

	key, err := c.rsaDecrypt(msg.Key)
	if err != nil {
		c.err("%v", err)
		return
	}

	from, err := c.aesDecrypt(key, msg.From)
	if err != nil {
		c.err("%v", err)
		return
	}

	text, err := c.aesDecrypt(key, msg.Text)
	if err != nil {
		c.err("%v", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.trunc()
	fmt.Print("\033[37m")
	fmt.Print("\rFrom: ")
	fmt.Print("\033[0m") // unset color choice
	fmt.Println(string(from))
	fmt.Print("\033[90m# ")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("\033[0m")
	fmt.Println(string(text))
	c.renderLine()
}

func (c *Client) readTextBlock() ([]rune, error) {
	// god dammit what have i gotten myself into
	msg := make([]rune, 0, 400)
	fmt.Print("\033[1K") // clear to beginning of current line
	fmt.Print("\r")      // move to beginning of current line
	fmt.Print("\033[s")  // save the cursor position
	renderMsg := func() {
		fmt.Print("\033[u")           // restore cursor position
		fmt.Print("\033[0J")          // clear to screen end
		fmt.Printf("%s", string(msg)) // write message out
	}
	in := bufio.NewReader(os.Stdin)
	for {
		r, _, err := in.ReadRune()
		switch err {
		case io.EOF:
			return msg, nil
		case nil:
		default:
			return nil, fmt.Errorf("error reading textblock: %v", err)
		}
		if unicode.IsGraphic(r) {
			msg = append(msg, r)
			renderMsg()
			continue
		}
		switch r {
		case 13: // enter
			msg = append(msg, '\n')
			renderMsg()
		case 127: // backspace
			if len(msg) == 0 {
				break
			}
			msg = msg[:len(msg)-1]
			renderMsg()
		case 4: // ctrl+d
			return msg, nil
		}
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

func (c *Client) aesKey() ([]byte, error) {
	return randslice(aes.BlockSize)
}

func (c *Client) aesDecrypt(key []byte, ctxt []byte) ([]byte, error) {
	c.info("aes decrypting...")
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("unable to create aes cipher: %v", err)
	}
	iv := ctxt[:aes.BlockSize]
	c.info("aes iv: %x", iv)

	ptxt := make([]byte, len(ctxt)-aes.BlockSize)
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ptxt, ctxt[aes.BlockSize:])
	return ptxt, nil
}

func (c *Client) aesEncrypt(key []byte, ptxt []byte) ([]byte, error) {
	c.info("aes encrypting...")
	if len(ptxt)%aes.BlockSize != 0 {
		pad := aes.BlockSize - len(ptxt)%aes.BlockSize
		c.info("padding by %d bytes", pad)
		// this is shitty.  There's a better way to do this, right?
		// this is also not reversible so I have to do this better.
		for i := 0; i < pad; i++ {
			ptxt = append(ptxt, ' ')
		}
	}

	c.info("new block cipher")
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("couldn't encrypt note: failed to make aes cipher: %v", err)
	}

	ctxt := make([]byte, aes.BlockSize+len(ptxt))
	iv := ctxt[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("couldn't encrypt note: failed to make aes iv: %v", err)
	}
	c.info("aes iv: %x", iv)

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ctxt[aes.BlockSize:], ptxt)
	c.info("aes encryption done")
	return ctxt, nil
}

func (c *Client) rsaDecrypt(ctext []byte) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, c.key, ctext)
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
		key:         key,
		host:        options.host,
		port:        options.port,
		nick:        options.nick,
		done:        make(chan interface{}),
		line:        make([]rune, 0, 32),
		keyStore:    make(map[string]rsa.PublicKey, 8),
		outstanding: make(map[int]chan Envelope),
	}
	client.run()
}

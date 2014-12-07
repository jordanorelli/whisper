package main

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	"io"
	"net"
	"strconv"
	"strings"
)

func stream(r io.Reader, c chan Envelope, e chan error, done chan interface{}) {
	defer close(done)
	decoder := json.NewDecoder(r)
	var env Envelope
	for {
		err := decoder.Decode(&env)
		switch err {
		case io.EOF:
			return
		case nil:
			c <- env
		default:
			e <- err
		}
	}
}

type serverConnection struct {
	conn net.Conn
	nick string
	key  rsa.PublicKey
	db   *leveldb.DB
}

func (s *serverConnection) sendMeta(template string, args ...interface{}) {
	m := Meta(fmt.Sprintf(template, args...))
	if err := s.sendRequest(m); err != nil {
		error_log.Printf("error sending message to client: %v", err)
	}
}

func (s *serverConnection) sendRequest(r request) error {
	return writeRequest(s.conn, r)
}

func (s *serverConnection) handleRequest(request Envelope) error {
	switch request.Kind {
	case "auth":
		return s.handleAuthRequest(request.Body)
	case "note":
		return s.handleNoteRequest(request.Body)
	default:
		return fmt.Errorf("no such request type: %v", request.Kind)
	}
}

func (s *serverConnection) handleAuthRequest(body json.RawMessage) error {
	var auth Auth
	if err := json.Unmarshal(body, &auth); err != nil {
		return fmt.Errorf("bad auth request: %v", err)
	}
	s.nick = auth.Nick
	s.key = auth.Key
	s.sendMeta("hello, %s", auth.Nick)
	if err := s.openDB(); err != nil {
		error_log.Printf("failed to open database: %v", err)
	}
	info_log.Printf("authenticated user %s", auth.Nick)
	return nil
}

func (s *serverConnection) handleNoteRequest(body json.RawMessage) error {
	r := &util.Range{Start: []byte("notes/")}
	it := s.db.NewIterator(r, nil)
	defer it.Release()

	id := 0
	if it.Last() {
		k := it.Key()
		id_s := strings.TrimPrefix(string(k), "notes/")
		lastId, err := strconv.Atoi(id_s)
		if err != nil {
			return fmt.Errorf("error getting note id: %v", err)
		}
		id = lastId + 1
	}
	key := fmt.Sprintf("notes/%d", id)
	if err := s.db.Put([]byte(key), body, nil); err != nil {
		return fmt.Errorf("unable to write note to db: %v", err)
	}
	info_log.Printf("stored new note at %s", key)
	return nil
}

func (s *serverConnection) openDB() error {
	path := fmt.Sprintf("./%s.db", s.nick)
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return fmt.Errorf("unable to open db file at %s: %v", path, err)
	}
	info_log.Printf("opened database file: %s", path)
	s.db = db
	return nil
}

func (s *serverConnection) run() {
	defer func() {
		s.conn.Close()
		info_log.Printf("connection ended: %v", s.conn.RemoteAddr())
	}()
	info_log.Printf("connection start: %v", s.conn.RemoteAddr())
	requests := make(chan Envelope)
	errors := make(chan error)
	done := make(chan interface{})
	go stream(s.conn, requests, errors, done)
	for {
		select {
		case request := <-requests:
			if err := s.handleRequest(request); err != nil {
				error_log.Printf("client error: %v", err)
			}
		case err := <-errors:
			error_log.Printf("connection error: %v", err)
		case <-done:
			return
		}
	}
}

func serve() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", options.port))
	if err != nil {
		exit(1, "couldn't open tcp port for listening: %v", err)
	}
	info_log.Printf("server listening: %s:%d", options.host, options.port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			error_log.Printf("error accepting new connection: %v", err)
			continue
		}
		srvConn := serverConnection{
			conn: conn,
		}
		go srvConn.run()
	}
}

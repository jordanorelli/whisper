package main

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	"io"
	"net"
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
	key  *rsa.PublicKey
	db   *userdb
}

func (s *serverConnection) sendResponse(id int, r request) error {
	return writeRequest(s.conn, id, r)
}

func (s *serverConnection) handleRequest(request Envelope) error {
	info_log.Printf("handle request #%d", request.Id)
	switch request.Kind {
	case "auth":
		return s.handleAuthRequest(request.Id, request.Body)
	case "note":
		return s.handleNoteRequest(request.Id, request.Body)
	case "get-note":
		return s.handleGetNoteRequest(request.Id, request.Body)
	case "list-notes":
		return s.handleListNotesRequest(request.Id, request.Body)
	case "key":
		return s.handleKeyRequest(request.Id, request.Body)
	case "message":
		return s.handleMessageRequest(request.Id, request.Body)
	case "get-message":
		return s.handleGetMessageRequest(request.Id, request.Body)
	case "list-messages":
		return s.handleListMessagesRequest(request.Id, request.Body)
	default:
		return fmt.Errorf("no such request type: %v", request.Kind)
	}
}

func (s *serverConnection) handleAuthRequest(requestId int, body json.RawMessage) error {
	var auth Auth
	if err := json.Unmarshal(body, &auth); err != nil {
		return fmt.Errorf("bad auth request: %v", err)
	}
	if auth.Nick == "" {
		return fmt.Errorf("empty username")
	}
	s.nick = auth.Nick
	if auth.Key == nil {
		return fmt.Errorf("empty key")
	}
	s.key = auth.Key
	if err := s.openDB(); err != nil {
		return fmt.Errorf("failed to open user database: %v", err)
	}
	b, err := s.db.Get([]byte("public_key"), nil)
	switch err {
	case leveldb.ErrNotFound:
		keybytes, err := json.Marshal(auth.Key)
		if err != nil {
			return fmt.Errorf("cannot marshal auth key: %v", err)
		}
		if err := s.db.Put([]byte("public_key"), keybytes, nil); err != nil {
			return fmt.Errorf("cannot write public key to database: %v", err)
		}
		info_log.Printf("saved key for user %s", auth.Nick)
	case nil:
		var key rsa.PublicKey
		if err := json.Unmarshal(b, &key); err != nil {
			return fmt.Errorf("cannot unmarshal auth key from request: %v", err)
		}
		if auth.Key.E != key.E {
			return fmt.Errorf("client presented wrong auth key")
		}
		if auth.Key.N.Cmp(key.N) != 0 {
			return fmt.Errorf("client presented wrong auth key")
		}
	default:
		return fmt.Errorf("unable to read public key: %v", err)
	}
	info_log.Printf("%s", b)
	info_log.Printf("authenticated user %s", auth.Nick)
	s.sendResponse(requestId, Bool(true))
	return nil
}

func (s *serverConnection) handleNoteRequest(requestId int, body json.RawMessage) error {
	r := util.BytesPrefix([]byte("notes/"))
	it := s.db.NewIterator(r, nil)
	defer it.Release()

	id := 0
	if it.Last() {
		k := it.Key()
		id_s := strings.TrimPrefix(string(k), "notes/")
		lastId, err := decodeInt(id_s)
		if err != nil {
			return fmt.Errorf("error getting note id: %v", err)
		}
		id = lastId + 1
	}
	key := fmt.Sprintf("notes/%s", encodeInt(id))
	if err := s.db.Put([]byte(key), body, nil); err != nil {
		return fmt.Errorf("unable to write note to db: %v", err)
	}
	info_log.Printf("stored new note at %s", key)
	return nil
}

func (s *serverConnection) handleGetNoteRequest(requestId int, body json.RawMessage) error {
	var req GetNoteRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return fmt.Errorf("bad getnote request: %v", err)
	}
	key := fmt.Sprintf("notes/%s", encodeInt(int(req)))
	b, err := s.db.Get([]byte(key), nil)
	if err != nil {
		return fmt.Errorf("couldn't retrieve note: %v", err)
	}
	var note EncryptedNote
	if err := json.Unmarshal(b, &note); err != nil {
		return fmt.Errorf("couldn't unmarshal note: %v", err)
	}
	return s.sendResponse(requestId, note)
}

func (s *serverConnection) handleListNotesRequest(requestId int, body json.RawMessage) error {
	r := util.BytesPrefix([]byte("notes/"))

	it := s.db.NewIterator(r, nil)
	defer it.Release()

	notes := make(ListNotesResponse, 0, 10)
	it.Last()
	for i := 0; it.Valid() && i < 10; i++ {
		key, val := it.Key(), it.Value()

		info_log.Printf("note %d has key %s", i, string(key))
		id_s := strings.TrimPrefix(string(key), "notes/")
		info_log.Printf("note id_s: %s", id_s)
		id, err := decodeInt(id_s)
		if err != nil {
			error_log.Printf("unable to parse note key %s: %v", id_s, err)
			it.Prev()
			continue
		}
		info_log.Printf("note key: %s id: %d\n", key, id)

		var note EncryptedNote
		if err := json.Unmarshal(val, &note); err != nil {
			error_log.Printf("unable to unmarshal encrypted note: %v", err)
			it.Prev()
			continue
		}
		notes = append(notes, ListNotesResponseItem{
			Id:    id,
			Key:   note.Key,
			Title: note.Title,
		})
		it.Prev()
	}
	if err := it.Error(); err != nil {
		return fmt.Errorf("error reading listnotes from db: %v", err)
	}
	return s.sendResponse(requestId, notes)
}

func (s *serverConnection) handleKeyRequest(requestId int, body json.RawMessage) error {
	var req KeyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		error_log.Printf("unable to read key request: %v", err)
		return err
	}
	info_log.Printf("get key: %v", req.Nick())
	key, err := getUserKey(req.Nick())
	if err != nil {
		return err
	}
	res := KeyResponse{
		Nick: req.Nick(),
		Key:  *key,
	}
	return s.sendResponse(requestId, res)
}

func (s *serverConnection) handleMessageRequest(requestId int, body json.RawMessage) error {
	var req Message
	if err := json.Unmarshal(body, &req); err != nil {
		error_log.Printf("unable to read message request: %v", err)
		return err
	}

	db, err := getUserDB(req.To, false)
	if err != nil {
		return err
	}

	k, err := db.nextKey("messages/")
	if err != nil {
		return fmt.Errorf("unable to save message: %v", err)
	}

	if err := db.Put([]byte(k), body, nil); err != nil {
		return fmt.Errorf("unable to save message: %v", err)
	}
	return s.sendResponse(requestId, Bool(true))
}

func (s *serverConnection) handleGetMessageRequest(requestId int, body json.RawMessage) error {
	var req GetMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return fmt.Errorf("unable to read getmessage request: %v", err)
	}

	key := fmt.Sprintf("messages/%s", encodeInt(req.Id))
	val, err := s.db.Get([]byte(key), nil)
	if err != nil {
		return fmt.Errorf("unable to read message: %v", err)
	}

	var msg Message
	if err := json.Unmarshal(val, &msg); err != nil {
		return fmt.Errorf("unable to parse message: %v", err)
	}
	return s.sendResponse(requestId, msg)
}

func (s *serverConnection) handleListMessagesRequest(requestId int, body json.RawMessage) error {
	var req ListMessages
	if err := json.Unmarshal(body, &req); err != nil {
		error_log.Printf("unable to read message request: %v", err)
		return err
	}

	prefix := []byte("messages/")
	messages := make(ListMessagesResponse, 0, 10)
	fn := func(n int, v []byte) error {
		var msg Message
		if err := json.Unmarshal(v, &msg); err != nil {
			return fmt.Errorf("unable to parse message blob: %v", err)
		}
		messages = append(messages, ListMessagesResponseItem{
			Id:   n,
			Key:  msg.Key,
			From: msg.From,
		})
		return nil
	}
	if err := s.db.collect(prefix, -10, fn); err != nil {
		return fmt.Errorf("error handling listmessages request: %v", err)
	}
	return s.sendResponse(requestId, messages)
}

func (s *serverConnection) openDB() error {
	db, err := getUserDB(s.nick, true)
	if err != nil {
		return err
	}
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
				s.sendResponse(request.Id, ErrorDoc(err.Error()))
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

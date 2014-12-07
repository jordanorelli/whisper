package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
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

func handleAuthRequest(conn net.Conn, body json.RawMessage) error {
	var auth Auth
	if err := json.Unmarshal(body, &auth); err != nil {
		return fmt.Errorf("bad auth request: %v", err)
	}
	info_log.Printf("authenticated user %s", auth.Nick)
	return nil
}

func handleRequest(conn net.Conn, request Envelope) error {
	switch request.Kind {
	case "auth":
		return handleAuthRequest(conn, request.Body)
	default:
		return fmt.Errorf("no such request type: %v", request.Kind)
	}
}

func handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		info_log.Printf("connection ended: %v", conn.RemoteAddr())
	}()
	info_log.Printf("connection start: %v", conn.RemoteAddr())
	requests := make(chan Envelope)
	errors := make(chan error)
	done := make(chan interface{})
	go stream(conn, requests, errors, done)
	for {
		select {
		case request := <-requests:
			if err := handleRequest(conn, request); err != nil {
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
		go handleConnection(conn)
	}
}

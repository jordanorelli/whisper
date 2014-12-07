package main

import (
	"encoding/json"
	"fmt"
	"net"
)

func authConnection(conn net.Conn) error {
	var raw json.RawMessage
	d := json.NewDecoder(conn)
	if err := d.Decode(&raw); err != nil {
		return fmt.Errorf("unable to decode client request: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("man, fuck all this. %v", err)
	}
	switch env.Kind {
	case "auth":
		var auth Auth
		if err := json.Unmarshal(env.Body, &auth); err != nil {
			return fmt.Errorf("unable to decode auth body: %v", err)
		}
		info_log.Printf("authenticated user %s", auth.Nick)
	}
	return nil
}

func handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		info_log.Printf("connection ended: %v", conn.RemoteAddr())
	}()
	info_log.Printf("connection start: %v", conn.RemoteAddr())
	authConnection(conn)
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

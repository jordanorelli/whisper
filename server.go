package main

import (
	// "crypto/rsa"
	"encoding/json"
	"fmt"
	"net"
)

func handleConnection(conn net.Conn) {
	defer conn.Close()
	d := json.NewDecoder(conn)

	// var nick string
	// var key rsa.PublicKey

	var env Envelope
	for {
		if err := d.Decode(&env); err != nil {
			error_log.Printf("unable to decode client request: %v", err)
			return
		}
		switch env.Kind {
		case "auth":
			var auth Auth
			if err := json.Unmarshal(env.Body, &auth); err != nil {
				error_log.Printf("unable to decode auth body: %v", err)
				break
			}
			// nick = auth.Nick
			// key = auth.Key
		}
	}
}

func serve() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", options.port))
	if err != nil {
		exit(1, "couldn't open tcp port for listening: %v", err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			error_log.Printf("error accepting new connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

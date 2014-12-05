package main

import (
	"encoding/json"
	"fmt"
	"net"
)

type Envelope struct {
	Kind string
	Body json.RawMessage
}

type request interface {
	Kind() string
}

func encodeRequest(conn net.Conn, r request) {
	b, err := json.Marshal(r)
	if err != nil {
		exit(1, "unable to encode client request body: %v", err)
	}
	e := Envelope{
		Kind: r.Kind(),
		Body: b,
	}
	if err := json.NewEncoder(conn).Encode(e); err != nil {
		exit(1, "unable to encode client request: %v", err)
	}
}

func decodeRequest(conn net.Conn) (request, error) {
	d := json.NewDecoder(conn)
	var env Envelope
	if err := d.Decode(&env); err != nil {
		return nil, fmt.Errorf("unable to decode client request: %v", err)
	}
	switch env.Kind {
	case "auth":
		var auth Auth
		if err := json.Unmarshal(env.Body, &auth); err != nil {
			return nil, fmt.Errorf("unable to unmarshal auth request: %v", err)
		}
		return &auth, nil
	default:
		return nil, fmt.Errorf("unknown request type: %s", env.Kind)
	}
}

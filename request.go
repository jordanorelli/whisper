package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
)

type Envelope struct {
	Id   int
	Kind string
	Body json.RawMessage
}

type request interface {
	Kind() string
}

func wrapRequest(id int, r request) (*Envelope, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("unable to wrap request: %v", err)
	}
	return &Envelope{
		Id:   id,
		Kind: r.Kind(),
		Body: b,
	}, nil
}

func writeRequest(w io.Writer, id int, r request) error {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("unable to marshal request: %v", err)
	}
	msg := json.RawMessage(b)
	e := &Envelope{
		Id:   id,
		Kind: r.Kind(),
		Body: msg,
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("unable to marshal request envelope: %v", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("unable to write request: %v", err)
	}
	return nil
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

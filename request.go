package main

import (
	"encoding/json"
	"fmt"
	"io"
)

var requestTypes = make(map[string]func() request, 32)

func registerRequestType(fn func() request) {
	r := fn()
	if _, ok := requestTypes[r.Kind()]; ok {
		panic("request type already registered")
	}
	requestTypes[r.Kind()] = fn
}

type Envelope struct {
	Id   int             `json:"id"`
	Kind string          `json:"kind"`
	Body json.RawMessage `json:"body"`
}

func (e Envelope) Open() (request, error) {
	fn, ok := requestTypes[e.Kind]
	if !ok {
		return nil, fmt.Errorf("unknown request type: %s", e.Kind)
	}
	r := fn()
	if err := json.Unmarshal(e.Body, r); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json in note open: %v", err)
	}
	return r, nil
}

// Bool is used to acknowledge that a request has been received and that there
// is no useful information for the user.
type Bool bool

func (b Bool) Kind() string {
	return "bool"
}

func init() { registerRequestType(func() request { return new(Bool) }) }

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

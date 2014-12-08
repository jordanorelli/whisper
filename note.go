package main

import (
	"crypto/rand"
)

type GetNoteRequest int

func (g GetNoteRequest) Kind() string {
	return "get-note"
}

type Note struct {
	Title string
	Body  []byte
}

type EncryptedNote struct {
	Key  []byte
	Body []byte
}

func (n EncryptedNote) Kind() string {
	return "note"
}

func randslice(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

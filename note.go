package main

import (
	"crypto/rand"
	"github.com/jordanorelli/lexnum"
)

var numEncoder = lexnum.NewEncoder('=', '-')

func encodeInt(n int) string {
	return numEncoder.EncodeInt(n)
}

func decodeInt(s string) (int, error) {
	return numEncoder.DecodeInt(s)
}

type GetNoteRequest struct {
	Id int
}

func (g GetNoteRequest) Kind() string {
	return "get-note"
}

func init() { registerRequestType(func() request { return new(GetNoteRequest) }) }

type Note struct {
	Title string
	Body  []byte
}

type EncryptedNote struct {
	Key   []byte
	Title []byte
	Body  []byte
}

func init() { registerRequestType(func() request { return new(EncryptedNote) }) }

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

type ListNotes struct {
	N int
}

func (l ListNotes) Kind() string {
	return "list-notes-request"
}

func init() { registerRequestType(func() request { return new(ListNotes) }) }

type ListNotesResponseItem struct {
	Id    int
	Key   []byte
	Title []byte
}

type ListNotesResponse []ListNotesResponseItem

func (l ListNotesResponse) Kind() string {
	return "list-notes-response"
}

func init() { registerRequestType(func() request { return new(ListNotesResponse) }) }

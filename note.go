package main

import ()

type NoteRequest []byte

func (n NoteRequest) Kind() string {
	return "note"
}

type NoteData struct {
	Title string
	Body  []byte
}

package main

import ()

type NoteRequest []byte

func (n NoteRequest) Kind() string {
	return "note"
}

type GetNoteRequest int

func (g GetNoteRequest) Kind() string {
	return "get-note"
}

type NoteData struct {
	Title string
	Body  []byte
}

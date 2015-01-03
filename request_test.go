package main

import (
	"crypto/rand"
	"crypto/rsa"
	"reflect"
	"testing"
)

var requests = []request{
	&Message{
		Key:  []byte("hmm maybe this should be checked."),
		From: []byte("bob"),
		To:   "alice",
		Text: []byte("this is my great message"),
	},
	&ListMessages{N: 10},
	&ListMessagesResponse{
		{0, []byte("key"), []byte("from")},
		{1, []byte("key"), []byte("from")},
		{2, []byte("key"), []byte("from")},
		{3, []byte("key"), []byte("from")},
	},
	&GetMessage{Id: 8},
	&GetNoteRequest{Id: 12},
	&EncryptedNote{
		Key:   []byte("this is not a key"),
		Title: []byte("likewise, this is not a title's ciphertext"),
		Body:  []byte("nor is this the ciphertext of an encrypted note"),
	},
	&ListNotes{N: 10},
	&ListNotesResponse{
		{0, []byte("key"), []byte("title")},
		{1, []byte("key"), []byte("title")},
		{2, []byte("key"), []byte("title")},
		{3, []byte("key"), []byte("title")},
	},
}

func TestEnvelope(t *testing.T) {
	t.Logf("envelope test")

	tru, falz := Bool(true), Bool(false)
	requests = append(requests, &tru, &falz)

	key, err := rsa.GenerateKey(rand.Reader, 128)
	if err != nil {
		t.Errorf("unable to create key for testing: %v", err)
	} else {
		requests = append(requests, &AuthRequest{
			Nick: "nick",
			Key:  &key.PublicKey,
		})
	}

	e := ErrorDoc("this is my error document.")
	requests = append(requests, &e)

	r := KeyRequest("bob")
	requests = append(requests, &r)

	key2, err := rsa.GenerateKey(rand.Reader, 128)
	if err != nil {
		t.Errorf("unable to create key for testing: %v", err)
	} else {
		requests = append(requests, &KeyResponse{
			Nick: "nick",
			Key:  key2.PublicKey,
		})
	}

	for i, r := range requests {
		t.Logf("wrapping request %d of type %v", i, reflect.TypeOf(r))
		e, err := wrapRequest(i, r)
		if err != nil {
			t.Errorf("unable to wrap %v request: %v", reflect.TypeOf(r), err)
			continue
		}
		r2, err := e.Open()
		if err != nil {
			t.Errorf("unable to open envelope %d of kind %v: %v", e.Id, e.Kind, err)
		}
		if !reflect.DeepEqual(r, r2) {
			t.Errorf("request didn't envelope and unenvelope correctly: %v != %v", r, r2)
		}
	}
}

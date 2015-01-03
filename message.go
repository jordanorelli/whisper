package main

import ()

type Message struct {
	Key  []byte
	From []byte
	To   string
	Text []byte
}

func (m Message) Kind() string {
	return "send-message"
}

type ListMessages struct {
	N int
}

func (l ListMessages) Kind() string {
	return "list-messages"
}

type ListMessagesResponseItem struct {
	Id   int
	Key  []byte
	From []byte
}

type ListMessagesResponse []ListMessagesResponseItem

func (l ListMessagesResponse) Kind() string {
	return "list-messages-response"
}

type GetMessage struct {
	Id int
}

func (g GetMessage) Kind() string {
	return "get-message"
}

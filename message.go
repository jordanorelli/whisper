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

func init() { registerRequestType(func() request { return new(Message) }) }

type ListMessages struct {
	N int
}

func (l ListMessages) Kind() string {
	return "list-messages"
}

func init() { registerRequestType(func() request { return new(ListMessages) }) }

type ListMessagesResponseItem struct {
	Id   int
	Key  []byte
	From []byte
}

type ListMessagesResponse []ListMessagesResponseItem

func (l ListMessagesResponse) Kind() string {
	return "list-messages-response"
}

func init() { registerRequestType(func() request { return new(ListMessagesResponse) }) }

type GetMessage struct {
	Id int
}

func (g GetMessage) Kind() string {
	return "get-message"
}

func init() { registerRequestType(func() request { return new(GetMessage) }) }

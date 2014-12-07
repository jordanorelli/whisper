package main

import ()

type Meta string

func (m Meta) Kind() string {
	return "meta"
}

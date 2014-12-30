package main

import ()

type ErrorDoc string

func (e ErrorDoc) Kind() string {
	return "error"
}

func (e ErrorDoc) Error() string {
	return string(e)
}

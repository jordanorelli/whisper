package main

import (
	"sync"
)

// connection manager
type CM struct {
	mu sync.Mutex
}

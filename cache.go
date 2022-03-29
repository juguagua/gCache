package gCache

import (
	"sync"
	"gCache"
)

type cache struct {
	mu  sync.Mutex
	lru *lru
}

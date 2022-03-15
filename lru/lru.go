package lru

import "container/list"

// Cache  lru Cache
type Cache struct {
	maxCap    int64      // Cap
	nbytes    int64      // currently used
	list      *list.List // double linked list
	cache     map[string]*list.Element
	OnEvicted func(key string, value Value) // callback function
}
type pair struct { // data type of the linked list node
	k     string
	value Value
}
type Value interface {
	Len() int
}

// New the constructor of cache
func New(cap int64, OnEvicted func(string, Value)) *Cache {
	return &Cache{
		maxCap:    cap,
		nbytes:    0,
		list:      list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: OnEvicted,
	}
}
func (c *Cache) Get(key string) (value Value, ok bool) {
	if el, ok := c.cache[key]; ok {
		c.list.MoveToFront(el)
		kv := el.Value.(*pair)
		return kv.value, true
	}
	return
}

// Add adds a value to the cache.
func (c *Cache) Put(key string, value Value) {
	if ele, ok := c.cache[key]; ok {
		c.list.MoveToFront(ele)
		kv := ele.Value.(*pair)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		ele := c.list.PushFront(&pair{key, value})
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	for c.maxCap != 0 && c.maxCap < c.nbytes {
		c.RemoveOldest()
	}
}

// RemoveOldest removes the oldest item
func (c *Cache) RemoveOldest() {
	ele := c.list.Back()
	if ele != nil {
		c.list.Remove(ele)
		kv := ele.Value.(*pair)
		delete(c.cache, kv.k)
		c.nbytes -= int64(len(kv.k)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			c.OnEvicted(kv.k, kv.value)
		}
	}
}

// Len the number of cache entries
func (c *Cache) Len() int {
	return c.list.Len()
}

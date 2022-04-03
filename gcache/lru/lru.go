package lru

import "container/list"

type Cache struct {
	cap      int64
	curBytes int64 // current size
	list     *list.List
	cache    map[string]*list.Element
	// optional and executed when an entry is purged
	OnEvicted func(key string, value Value)
}

type entry struct {
	key   string
	value Value
}

// Value use Len() to count how many bytes it takes
type Value interface {
	Len() int
}

// Constructor of Cache
func New(cap int64, onEvicted func(key string, value Value)) *Cache {
	return &Cache{
		cap:       cap,
		curBytes:  0,
		list:      list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

func (c *Cache) Get(key string) (value Value, ok bool) {
	if el, ok := c.cache[key]; ok {
		c.list.MoveToFront(el)
		kv := el.Value.(*entry)
		return kv.value, true
	}
	return
}

func (c *Cache) RemoveOldest() {
	el := c.list.Back()
	if el != nil {
		c.list.Remove(el)
		kv := el.Value.(*entry)
		delete(c.cache, kv.key)
		c.curBytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

func (c *Cache) Add(key string, value Value) {
	if el, ok := c.cache[key]; ok {
		c.list.MoveToFront(el)
		kv := el.Value.(*entry)
		c.curBytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		el := c.list.PushFront(&entry{key: key, value: value})
		c.cache[key] = el
		c.curBytes += int64(len(key)) + int64(value.Len())
	}
	for c.cap != 0 && c.cap < c.curBytes {
		c.RemoveOldest()
	}
}

func (c *Cache) Len() int {
	return c.list.Len()
}

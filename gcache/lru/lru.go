package lru

import "container/list"

type Cache struct {
	cap       int64                         // 缓存容量
	curBytes  int64                         // 当前的缓存大小
	list      *list.List                    // cache的队列（双向链表实现）
	cache     map[string]*list.Element      // 字典 映射
	OnEvicted func(key string, value Value) // 回调函数
}

// 实际的节点
type entry struct {
	key   string
	value Value
}

// 为了通用性，允许value是任何实现了Value接口的类型
type Value interface {
	// 用于返回value所占用的内存大小
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

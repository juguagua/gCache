package gcache

import (
	"gCache/gcache/lru"
	"sync"
)

// 实例化lru缓存，封装add和get方法
type cache struct {
	mu         sync.Mutex // 互斥锁
	lru        *lru.Cache // lru缓存
	cacheBytes int64      // 缓存容量
}

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil { // lazy initialization
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}

	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}
	return
}

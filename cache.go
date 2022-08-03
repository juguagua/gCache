package gcache

import (
	"github.com/juguagua/gCache/lru"
	"sync"
)

// cache 模块负责提供对lru模块的并发控制

// 这样设计可以进行cache和算法的分离，比如现在有多种缓存模块可选，只需替换cache成员即可
type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int
}

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (ByteView, bool) { // 注意：Get操作需要修改lru中的双向链表，需要使用互斥锁。
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return ByteView{}, false
	}
	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}
	return ByteView{}, false
}

func (c *cache) remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}
	c.lru.Remove(key)
}

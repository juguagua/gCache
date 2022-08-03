package lru

import (
	"container/list"
	"github.com/juguagua/gCache/zset"
	"time"
)

// 缓存算法对比：
// LRU：最近最少使用。它很综合，如果数据最近很少被使用，那么就会被淘汰。它的实现很简单，使用map+双向链表。
// LFU：最不经常使用。它根据访问次数来决定是否被淘汰，可能会存在某个一段时间很热的key在另外一段时间不那么热，却由于积累的访问次数过大而无法被淘汰。它的实现使用两个map+双向链表。https://juejin.cn/post/6987260805888606245#heading-2

// Warning: lru包不提供并发一致机制
// TODO: 实现lru-k

const (
	expiresZSetKey = ""
	// 每次移除过期键数量
	removeExpireN = 10
)

// Cache LRU缓存
type Cache struct {
	capacity  int // 缓存容量
	length    int // 当前缓存大小
	ll        *list.List
	cache     map[string]*list.Element
	onEvicted func(key string, value Value) // 可选，在entry被移除的时候执行
	expires   *zset.SortedSet               // 过期键集合
}

// Entry 定义双向链表节点所存储的对象
type Entry struct {
	key   string
	value Value
}

// Value 用于计算值占用了多少字节和过期时间
type Value interface {
	Len() int
	Expire() time.Time
}

// New 创建指定最大容量的LRU缓存。
// 当maxBytes为0时，代表cache无内存限制，无限存放
func New(maxBytes int, onEvicted func(key string, value Value)) *Cache {
	return &Cache{
		capacity:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		onEvicted: onEvicted,
		expires:   zset.New(),
	}
}

// Get 从缓存获取对应key的value
func (c *Cache) Get(key string) (value Value, ok bool) {
	element, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	ent := element.Value.(*Entry)
	// 移除过期的键
	if !ent.value.Expire().IsZero() && ent.value.Expire().Before(time.Now()) {
		c.removeElement(element)
		return nil, false
	}
	c.ll.MoveToBack(element)
	return ent.value, true
}

// Add 添加数据到缓存
func (c *Cache) Add(key string, value Value) {
	if element, ok := c.cache[key]; ok {
		c.ll.MoveToBack(element)
		ent := element.Value.(*Entry)
		c.length += value.Len() - ent.value.Len()
		ent.value = value
	} else {
		ent := &Entry{
			key:   key,
			value: value,
		}
		element := c.ll.PushBack(ent)
		c.cache[key] = element
		c.length += len(key) + value.Len()
	}
	// 如果有超时时间则设置
	if !value.Expire().IsZero() {
		c.expires.ZAdd(expiresZSetKey, value.Expire().UnixNano(), key)
	} else {
		// 没有则删除
		c.expires.ZRem(expiresZSetKey, key)
	}
	// 淘汰过期的key
	if c.capacity != 0 {
		c.removeExpire(removeExpireN)
	}

	// 淘汰最近最少访问的key
	for c.capacity != 0 && c.length > c.capacity {
		c.removeOldest()
	}
}

// Remove 移除某个键
func (c *Cache) Remove(key string) {
	if element, ok := c.cache[key]; ok {
		c.removeElement(element)
	}
}

// Len 返回数据数量
func (c *Cache) Len() int {
	return c.ll.Len()
}

// 移除最近最少访问的数据
func (c *Cache) removeOldest() {
	front := c.ll.Front()
	if front != nil {
		c.removeElement(front)
	}
}

// 移除指定键，并删除链表里面的节点，减少lru缓存大小，删除过期时间，调用回调函数
func (c *Cache) removeElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*Entry)
	delete(c.cache, kv.key)
	c.length -= len(kv.key) + kv.value.Len()
	// 移除过期键
	if !kv.value.Expire().IsZero() {
		c.expires.ZRem(expiresZSetKey, kv.key)
	}
	if c.onEvicted != nil {
		c.onEvicted(kv.key, kv.value)
	}
}

// 移除过期的键
// 返回未删除的数量
func (c *Cache) removeExpire(n int) int {
	for n > 0 && c.expires.ZCard(expiresZSetKey) > 0 {
		values := c.expires.ZRangeWithScores(expiresZSetKey, 0, 0)
		key, expireNano := values[0].(string), values[1].(int64)
		// 第一个键都没超时，结束循环
		if expireNano > time.Now().UnixNano() {
			break
		}
		c.Remove(key)
		n--
	}
	return n
}

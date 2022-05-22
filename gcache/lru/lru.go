package lru

import "container/list"

type Cache struct {
	cap      int64      // 缓存容量, 0表示无限制
	curBytes int64      // 已使用的缓存大小
	list     *list.List // cache的队列（双向链表实现）
	// 字典映射，通过字符串能拿到真实的缓存值entry，Element中的value是interface{}类型，即任何类型
	cache     map[string]*list.Element
	OnEvicted func(key string, value Value) // 回调函数
}

// 实际的节点：即list中的kv结果
type entry struct {
	key   string
	value Value
}

// Value 为了通用性，允许entry中的value是任何实现了Value接口的类型
type Value interface {
	Len() int // Len 用于返回value所占用的内存大小
}

// New Constructor of Cache
func New(cap int64, onEvicted func(key string, value Value)) *Cache {
	return &Cache{
		cap:       cap,
		curBytes:  0,
		list:      list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// Get 从lru缓存中得到数据
func (c *Cache) Get(key string) (value Value, ok bool) {
	if el, ok := c.cache[key]; ok {
		c.list.MoveToFront(el)  // 移动到队头
		kv := el.Value.(*entry) // 类型断言
		return kv.value, true
	}
	return
}

// RemoveOldest lru缓存淘汰
func (c *Cache) RemoveOldest() {
	tail := c.list.Back()
	if tail != nil {
		c.list.Remove(tail)
		kv := tail.Value.(*entry)
		delete(c.cache, kv.key)
		c.curBytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil { // 淘汰key时执行它的回调函数
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Add 往lru缓存中添加或者更新数据，注意如果超出容量要进行旧值移除
func (c *Cache) Add(key string, value Value) {
	if el, ok := c.cache[key]; ok { // 更新数据
		c.list.MoveToFront(el)
		kv := el.Value.(*entry)
		c.curBytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		el := c.list.PushFront(&entry{key: key, value: value})
		c.cache[key] = el
		c.curBytes += int64(len(key)) + int64(value.Len())
	}
	for c.cap != 0 && c.cap < c.curBytes { // 内存不足时要进行淘汰
		c.RemoveOldest()
	}
}

// Len 为了方便测试，实现Len方法来获取添加了多少数据
func (c *Cache) Len() int {
	return c.list.Len()
}

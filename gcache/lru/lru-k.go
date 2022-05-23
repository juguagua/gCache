package lru

import (
	"container/list"
	"time"
)

// item view
type ViewItem struct {
	key          string    // key值
	view         int       // 访问次数
	lastViewTime time.Time // 最后一次的访问时间
}

func NewViewItem(key string) *ViewItem {
	return &ViewItem{
		key:          key,
		view:         1,
		lastViewTime: time.Now(),
	}
}

// 更新视图，规则如下：
// - 访问时间间隔超过d，重置视图
// - 否则，访问++, 更新最后访问时间

// UpdateWithExpireExam 更新访问记录
func (this *ViewItem) UpdateWithExpireExam(d time.Duration) (view int, expired bool) {
	if expired = this.expired(d); expired { // 如果过期要重置
		this.Reset()
		view = this.view
		return
	}
	this.view++
	view = this.view
	this.lastViewTime = time.Now()
	return
}

// 判断当前ViewItem是否过期
func (this *ViewItem) expired(d time.Duration) bool {
	if time.Now().Sub(this.lastViewTime) > d && d != 0 {
		return true
	}
	return false
}

// Reset 重置ViewItem
func (this *ViewItem) Reset() *ViewItem {
	this.view = 1
	this.lastViewTime = time.Now()
	return this
}

// cache item

type CacheItem struct {
	view  *ViewItem
	value Value
}

func NewCacheItem(view *ViewItem, value Value) *CacheItem {
	return &CacheItem{
		value: value,
		view:  view,
	}
}

// history view

type ViewHistory struct {
	maxBytes  int64
	nBytes    int64
	k         int
	ll        *list.List
	expire    time.Duration
	onEvicted func(key string)
}

func NewViewHistory(maxBytes int64, k int, expire time.Duration, onEvicted func(string)) *ViewHistory {
	return &ViewHistory{
		maxBytes:  maxBytes,
		nBytes:    0,
		k:         k,
		ll:        list.New(),
		expire:    expire,
		onEvicted: onEvicted,
	}
}

func (this *ViewHistory) Add(key string) (new *list.Element) {
	view := NewViewItem(key)
	new = this.ll.PushFront(view) // 将当前访问到的key对应的view放到队头
	this.nBytes += int64(len(key))
	for this.maxBytes != 0 && this.maxBytes < this.nBytes {
		this.RemoveOldest()
	}
	return
}

func (this *ViewHistory) UpdateView(pointer *list.Element) (cached bool) {
	this.ll.Remove(pointer)
	// 如果 view > k, 则从history中删除加入到cache中
	if view, _ := pointer.Value.(*ViewItem).UpdateWithExpireExam(this.expire); view >= this.k {
		this.nBytes -= int64(len(pointer.Value.(*ViewItem).key))
		cached = true
		return
	}
	this.ll.PushFront(pointer.Value.(*ViewItem))
	return
}

func (this *ViewHistory) RemoveOldest() {
	if ele := this.ll.Back(); ele != nil {
		this.ll.Remove(ele)
		viewItem := ele.Value.(*ViewItem)
		this.nBytes -= int64(len(viewItem.key))
		if this.onEvicted != nil {
			this.onEvicted(viewItem.key)
		}
	}
}

type MemoryCache struct {
	maxBytes  int64
	nBytes    int64
	ll        *list.List
	expire    time.Duration
	onEvicted func(value Value)
}

func NewMemoryCache(maxBytes int64, expire time.Duration, onEvicted func(Value)) *MemoryCache {
	return &MemoryCache{
		maxBytes:  maxBytes,
		nBytes:    0,
		ll:        list.New(),
		expire:    expire,
		onEvicted: onEvicted,
	}
}

func (this *MemoryCache) Add(view *ViewItem, value Value) (new *list.Element) {
	new = this.ll.PushFront(NewCacheItem(view, value))
	// 淘汰机制
	if value != nil {
		this.nBytes += int64(new.Value.(*CacheItem).value.Len())
	}
	for this.maxBytes != 0 && this.maxBytes < this.nBytes {
		this.RemoveOldest()
	}
	return
}

func (this *MemoryCache) UpdateValue(pointer *list.Element, value Value) {
	pointer.Value.(*CacheItem).value = value
	this.nBytes += int64(value.Len()) - int64(pointer.Value.(*CacheItem).value.Len())
	// 增加淘汰时间
	this.ll.MoveToFront(pointer)
}

func (this *MemoryCache) UpdateView(pointer *list.Element) (expired bool) {
	this.ll.Remove(pointer)
	if _, expired = pointer.Value.(*CacheItem).view.UpdateWithExpireExam(this.expire); expired {
		if val := pointer.Value.(*CacheItem).value; val != nil {
			this.nBytes -= int64(val.Len())
		}
		return
	}
	this.ll.PushFront(pointer.Value.(*CacheItem))
	return
}

func (this *MemoryCache) RemoveOldest() {
	if ele := this.ll.Back(); ele != nil {
		this.ll.Remove(ele)
		cacheItem := ele.Value.(*CacheItem)
		this.nBytes -= int64(cacheItem.value.Len())
		if this.onEvicted != nil {
			this.onEvicted(cacheItem.value)
		}
	}
}

type MemoryCacheManager struct {
	history    *ViewHistory
	cache      *MemoryCache
	historyMap map[string]*list.Element
	cacheMap   map[string]*list.Element
}

func NewMemoryCacheManager(maxBytes int64, k int, expire time.Duration, onEvicted1 func(string), onEvicted2 func(Value)) *MemoryCacheManager {
	return &MemoryCacheManager{
		history:    NewViewHistory(maxBytes, k, expire, onEvicted1),
		cache:      NewMemoryCache(maxBytes, expire, onEvicted2),
		historyMap: make(map[string]*list.Element),
		cacheMap:   make(map[string]*list.Element),
	}
}

func (this *MemoryCacheManager) Get(key string) (value Value, isOk bool) {
	//* 可能key已经从history移除，但是没有historyMap；所以还需判断是否真正存在！
	if pointer, ok := this.historyMap[key]; ok && (pointer.Prev() != nil || pointer.Next() != nil) {
		if cached := this.history.UpdateView(pointer); cached {
			delete(this.historyMap, key)
			new := this.cache.Add(pointer.Value.(*ViewItem), nil)
			this.cacheMap[key] = new
		}
	} else if pointer, ok := this.cacheMap[key]; ok {
		expire := time.Now().Add(500 * time.Millisecond)
		for {
			if time.Now().After(expire) {
				break
			}
			if value = pointer.Value.(*CacheItem).value; value != nil {
				isOk = true
				break
			}
		}
		if expired := this.cache.UpdateView(pointer); expired {
			delete(this.cacheMap, key)
		}
	} else {
		new := this.history.Add(key)
		this.historyMap[key] = new
	}
	return
}

func (this *MemoryCacheManager) Add(key string, value Value) {
	var (
		pointer *list.Element
		ok      bool
	)
	if pointer, ok = this.cacheMap[key]; !ok {
		return
	}
	this.cache.UpdateValue(pointer, value)
}

package gcache

import (
	"fmt"
	"log"
	"sync"
)

// Group 一个Group为一个缓存的命名空间，可以按照缓存功能不同进行分类
type Group struct {
	name      string
	getter    Getter // 回调函数
	mainCache cache  // Group中并发缓存的具体实现
}

// Getter 回调函数的接口
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 实现了Getter接口的接口型函数
type GetterFunc func(key string) ([]byte, error)

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group) // 全局变量，用map存储所有的Group
)

// NewGroup 创建一个新的Group
func NewGroup(name string, getter Getter, cacheBytes int64) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
	}
	groups[name] = g // 将new的Group加入到全局变量中去
	return g
}

// GetGroup 返回特定名称的Group
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// Get 从指定group中查找key所对应的value
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	if v, ok := g.mainCache.get(key); ok {
		log.Println("gCache hit")
		return v, nil
	}
	return g.load(key)
}

// 获取数据并加载到缓存
func (g *Group) load(key string) (ByteView, error) {
	return g.getLocally(key)
}

// 数据不存在缓存中时，从数据源处获得数据
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

// 将数据加载到g缓存中
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

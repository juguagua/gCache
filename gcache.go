package gcache

import (
	"fmt"
	"github.com/juguagua/gCache/singleflight"

	"log"
	"sync"
	"time"
)

// gcache 模块提供比cache模块更高一层抽象的能力
// 换句话说，实现了填充缓存/命名划分缓存的能力

// Getter 要求对象实现从数据源获取数据的能力
type Getter interface {
	Get(key string) (ByteView, error) // 回调函数
}

// GetterFunc 函数类型实现Getter接口
type GetterFunc func(key string) (ByteView, error)

// Get 通过实现Get方法，使得任意匿名函数func
// 通过被GetterFunc(func)类型强制转换后，实现了 Getter 接口的能力
func (f GetterFunc) Get(key string) (ByteView, error) {
	return f(key)
}

// Group 提供命名管理缓存/填充缓存的能力
type Group struct {
	name             string               // 缓存空间的名字
	getter           Getter               // 数据源获取数据
	mainCache        *cache               // 主缓存，并发缓存
	hotCache         *cache               // 热点缓存
	server           Picker               // 用于获取远程节点请求客户端
	flight           *singleflight.Flight // 避免对同一个key多次加载造成缓存击穿
	emptyKeyDuration time.Duration        // getter返回error时对应空值key的过期时间
}

var (
	// 对全局group操作的锁
	mu sync.RWMutex
	// 缓存全局的group
	groups = make(map[string]*Group)
)

// NewGroup 创建一个新的缓存空间
func NewGroup(name string, cacheBytes int, getter Getter) *Group {
	if getter == nil { // 必须得有缓存未命中时的回调接口
		panic("nil Getter")
	}
	g := &Group{
		name:   name,
		getter: getter,
		mainCache: &cache{
			cacheBytes: cacheBytes,
		},
		flight: &singleflight.Flight{},
	}
	mu.Lock()
	defer mu.Unlock()
	groups[name] = g
	return g
}

// RegisterSvr 为 Group 注册 Server
func (g *Group) RegisterSvr(p Picker) {
	if g.server != nil {
		panic("group had been registered server")
	}
	g.server = p
}

// GetGroup 获取对应命名空间的缓存
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

func DestroyGroup(name string) {
	g := GetGroup(name)
	if g != nil {
		svr := g.server.(*server)
		svr.Stop()
		delete(groups, name)
		log.Printf("Destroy cache [%s %s]", name, svr.addr)
	}
}

// SetEmptyWhenError 当getter返回error时设置空值，缓解缓存穿透问题
// 为0表示该机制不生效
func (g *Group) SetEmptyWhenError(duration time.Duration) {
	g.emptyKeyDuration = duration
}

// SetHotCache 设置远程节点Hot Key-Value的缓存，避免频繁请求远程节点
func (g *Group) SetHotCache(cacheBytes int) {
	if cacheBytes <= 0 {
		panic("hot cache must be greater than 0")
	}
	g.hotCache = &cache{
		cacheBytes: cacheBytes,
	}
}

// Get 从缓存获取key对应的value
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok { // 先从主缓存获取
		log.Println("[Cache] main cache hit")
		return v, nil
	}
	if g.hotCache != nil {
		if v, ok := g.hotCache.get(key); ok { // 主缓存没有看热点缓存
			log.Println("[Cache] hot cache hit")
			return v, nil
		}
	}
	return g.load(key)
}

// 加载缓存
func (g *Group) load(key string) (ByteView, error) {
	view, err := g.flight.Fly(key, func() (interface{}, error) {
		if g.server != nil { // 先判断是否需要从远程加载
			if fetcher, ok := g.server.Pick(key); ok { // ok代表需要从远程加载
				view, err := fetcher.Fetch(g.name, key)
				if err == nil {
					g.populateCache(key, view, g.hotCache)
					return view, nil
				}
				log.Printf("[Cache] failed to get from peer key=%s, err=%v\n", key, err)
			}
		}
		// 否则从本地加载
		return g.loadLocally(key)
	})
	if err != nil {
		return ByteView{}, err
	}
	return view.(ByteView), nil
}

// 从本地节点加载缓存值
func (g *Group) loadLocally(key string) (ByteView, error) {
	value, err := g.getter.Get(key)
	if err != nil {
		if g.emptyKeyDuration == 0 {
			return ByteView{}, err
		}
		// 走缓存空值机制
		value = ByteView{
			expire: time.Now().Add(g.emptyKeyDuration),
		}
	}
	g.populateCache(key, value, g.mainCache)
	return value, nil
}

// 从本地节点删除缓存
func (g *Group) removeLocally(key string) {
	g.mainCache.remove(key)
	if g.hotCache != nil {
		g.hotCache.remove(key)
	}
}

// 填充到缓存
func (g *Group) populateCache(key string, value ByteView, cache *cache) {
	if cache == nil {
		return
	}
	cache.add(key, value)
}

package gcache

import "C"
import (
	"fmt"
	pb "gCache/gcache/gcachepb/gcachepb"
	"gCache/gcache/singleflight"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// 每分钟获取的QPS上限
const maxMinuteRemoteQPS = 10

// Getter 回调函数接口，如果缓存不存在，执行回调函数得到源数据（数据源应由用户决定）
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 实现了Getter接口的接口型函数，方便调用者将匿名函数转换为接口传参
// 传递接口而不直接传函数的原因是传递接口更加的通用，可拓展性强，比如在接口中要增加方法，如果传递的是函数就得重新写
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// Group 一个Group为一个缓存的命名空间，可以按照缓存功能不同进行分类
type Group struct {
	name      string               // 缓存空间名
	getter    Getter               // 回调函数
	mainCache cache                // Group真实的缓存存储
	hotCache  cache                // 热点缓存
	peers     PeerPicker           // 提供选择节点方法
	loader    *singleflight.Group  // 使用singleflight来确保每个key一次只会发起一个请求
	keys      map[string]*KeyStats // key对应的统计信息map
}

// AtomicInt 封装一个原子类
type AtomicInt int64

// Add 原子自增
func (i *AtomicInt) Add(n int64) {
	atomic.AddInt64((*int64)(i), n)
}

func (i *AtomicInt) Get() int64 {
	return atomic.LoadInt64((*int64)(i))
}

// KeyStats Key的统计信息
type KeyStats struct {
	firstGetTime time.Time
	remoteCnt    AtomicInt //利用atomic包封装的原子类，远程调用次数
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group) // 全局变量，用map存储所有的Group
)

// NewGroup 创建一个新的Group
func NewGroup(name string, getter Getter, cacheBytes int64) *Group {
	if getter == nil {
		panic("nil Getter") // 回调函数是必要的
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes * 7 / 8},
		hotCache:  cache{cacheBytes: cacheBytes / 8},
		loader:    &singleflight.Group{},
		keys:      map[string]*KeyStats{},
	}
	groups[name] = g // 将new的Group加入到全局变量中去
	return g
}

// GetGroup 根据Group name返回指定的Group
func GetGroup(name string) *Group {
	mu.RLock() // 对于只读操作只需要用读锁就可以
	g := groups[name]
	mu.RUnlock()
	return g
}

// Get 在指定group中查找key所对应的缓存值
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	if v, ok := g.mainCache.get(key); ok { // 调用cache的get方法得到缓存值
		log.Println("gCache hit")
		return v, nil
	}
	// add hotCache
	if v, ok := g.hotCache.get(key); ok {
		log.Printf("[GaCache (hotCache)] hit")
		return v, nil
	}
	return g.load(key) // 如果在group缓存中没有，就从数据源处获取并加载到缓存中
}

// 获取数据并加载到缓存，若非本机节点则通过getFromPeer从远程节点获取
func (g *Group) load(key string) (value ByteView, err error) {
	viewi, err := g.loader.Do(key, func() (interface{}, error) { // 将load的逻辑包在loader.Do中确保并发请求不会一起调用
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok { // 先从远程节点中选取相应的节点
				if value, err = g.getFromPeer(peer, key); err != nil { // 访问远程节点，获取缓存值
					return value, nil
				}
				log.Println("failed to get from peer", err)
			}
		}
		return g.getLocally(key) // 如果不在远程节点，就从本地节点获取
	})
	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

// 从本地数据源获取数据
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key) // 执行回调函数从数据源获取数据
	if err != nil {
		return ByteView{}, err
	}
	// 将数据源的数据拷贝一份放入cache中，防止其他外部程序访问该数据进行修改
	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value, &g.mainCache)
	return value, nil
}

// getFromPeer 用实现了PeerGetter接口的httpGetter访问远程节点，获取缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{ // 构建proto的message
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res) // 在peer远程节点的group中查找缓存值
	if err != nil {
		return ByteView{}, err
	}
	//远程获取cnt++
	if stat, ok := g.keys[key]; ok {
		stat.remoteCnt.Add(1)
		//计算QPS
		interval := float64(time.Now().Unix()-stat.firstGetTime.Unix()) / 60
		qps := stat.remoteCnt.Get() / int64(math.Max(1, math.Round(interval)))
		if qps >= maxMinuteRemoteQPS {
			//存入hotCache
			g.populateCache(key, ByteView{b: res.Value}, &g.hotCache)
			//删除映射关系,节省内存
			mu.Lock()
			delete(g.keys, key)
			mu.Unlock()
		}
	} else {
		//第一次获取
		g.keys[key] = &KeyStats{
			firstGetTime: time.Now(),
			remoteCnt:    1,
		}
	}
	return ByteView{b: res.Value}, nil
}

// 将数据加载到cache中(包括mainCache 和 hotCache)
func (g *Group) populateCache(key string, value ByteView, c *cache) {
	c.add(key, value)
}

// RegisterPeers 将实现了PeerPicker接口的HTTPPool注入到Group中用于group的通信
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

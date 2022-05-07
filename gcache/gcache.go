package gcache

import (
	"fmt"
	"log"
	"sync"
)

// Group 一个Group为一个缓存的命名空间，可以按照缓存功能不同进行分类
type Group struct {
	name      string     // 缓存空间名
	getter    Getter     // 回调函数
	mainCache cache      // Group真实的缓存存储
	peers     PeerPicker // 提供选择节点方法
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
		mainCache: cache{cacheBytes: cacheBytes},
	}
	groups[name] = g // 将new的Group加入到全局变量中去
	return g
}

// GetGroup 根据Group name返回指定的Group
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// Getter 回调函数接口，如果缓存不存在，执行回调函数得到源数据（数据源应由用户决定）
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 实现了Getter接口的接口型函数
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
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
	return g.load(key) // 如果在group缓存中没有，就从数据源处获取并加载到缓存中
}

// 获取数据并加载到缓存，若非本机节点则通过getFromPeer从远程节点获取
func (g *Group) load(key string) (value ByteView, err error) {
	if g.peers != nil {
		if peer, ok := g.peers.PickPeer(key); ok { // 先从远程节点中选取相应的节点
			if value, err = g.getFromPeer(peer, key); err != nil { // 访问远程节点，获取缓存值
				return value, nil
			}
			log.Println("failed to get from peer", err)
		}
	}
	return g.getLocally(key) // 如果不在远程节点，就从本地节点获取
}

// 从本地数据源获取数据
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

// RegisterPeers 将实现了PeerPicker接口的HTTPPool注入到Group中用于group的通信
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// getFromPeer 用实现了PeerGetter接口的httpGetter访问远程节点，获取缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key) // 在peer远程节点的group中查找缓存值
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
}

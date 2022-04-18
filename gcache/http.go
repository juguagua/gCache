package gcache

import (
	"fmt"
	"gCache/gcache/consistenthash"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const (
	defaultBasePath = "/_gcache/"
	defaultReplicas = 50
)

type HTTPPool struct {
	self        string // 用来记录自己的地址，包括主机名/IP/端口号
	basePath    string // 节点间通讯地址的前缀
	mu          sync.Mutex
	peers       *consistenthash.Map    // 根据key来选择节点
	httpGetters map[string]*httpGetter // 映射远程节点地址与对应的httpGetter
}

// http 客户端
type httpGetter struct {
	baseURL string // 表示将要访问的远程节点的地址
}

// Get 获取返回值
func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	u := fmt.Sprintf(
		"%v%v%v",
		h.baseURL,
		url.QueryEscape(group),
		url.QueryEscape(key),
	)
	res, err := http.Get(u) // 给指定的url发送一个get请求，拿到返回值
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body) // 解析响应
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}

	return bytes, nil
}

var _ PeerGetter = (*httpGetter)(nil) // 提供编译器静态检查，判断httpGetter是否实现了PeerGetter这个接口（类型断言）

func (p *HTTPPool) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	//TODO implement me
	panic("implement me")
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServerHTTP 处理所有http请求
func (p *HTTPPool) ServerHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	} // 判断url是否包含节点通讯地址指定的前缀
	p.Log("%s %s", r.Method, r.URL.Path)

	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2) // 分割url
	if len(parts) != 2 {                                          // 如果分割后的url不符合规则，则返回错误信息
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	groupName, key := parts[0], parts[1]
	group := GetGroup(groupName) // 获得指定的缓存group
	if group == nil {
		http.Error(w, "no such group", http.StatusNotFound)
		return
	}
	view, err := group.Get(key) // 得到请求的缓存数据
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(view.ByteSlice()) // 写到响应中返回
}

// 实例化一致性哈希算法和添加节点
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers { // 为每个节点绑定一个http客户端
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// 包装一致性哈希的Get方法，根据key选择具体的节点并返回节点对应的客户端
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

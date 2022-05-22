package gcache

import (
	"fmt"
	"gCache/gcache/consistenthash"
	pb "gCache/gcache/gcachepb/gcachepb"
	"google.golang.org/protobuf/proto"
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

// 节点间http通信的核心结构
type HTTPPool struct {
	self        string // 用来记录自己节点的地址，包括主机名/IP/端口号
	basePath    string // 节点间通讯地址的指定前缀， e.g. "https://example.net:8000"
	mu          sync.Mutex
	peers       *consistenthash.Map    // 一致性哈希的实例，根据key来选择节点
	httpGetters map[string]*httpGetter // 映射远程节点地址与对应的httpGetter(节点的ip:port/defaultPath), 因为httpGetter与远程节点的地址有关
}

// NewHTTPPool 创建一个HTTPPool
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// http 客户端，用于向远程节点请求数据，可以理解为存远程节点的地址的结构
type httpGetter struct {
	baseURL string // 表示将要访问的远程节点的地址
}

// Get httpGetter实现peerGetter接口作为客户端，get方法通过节点地址和groupName和key请求数据
func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	u := fmt.Sprintf(
		"%v%v%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)
	res, err := http.Get(u) // 给指定的远程地址发送一个get请求，拿到返回值
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body) // 解析响应
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	// 解码proto并将结果存入out中
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	return nil
}

// http 服务端
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	} // 判断url是否包含节点通讯地址指定的前缀
	p.Log("%s %s", r.Method, r.URL.Path)
	// /<basepath>/<groupname>/<key> required
	// 以‘/’为界限将groupName和key划分为2个part
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1] // 得到参数信息

	group := GetGroup(groupName) // 获得指定的缓存group
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	view, err := group.Get(key) // 从该缓存group中得到请求的缓存数据
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 使用proto对http响应进行编码
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body) // 用w.Write()将缓存值作为httpResponse的body返回
}

// Log 带有服务器名称的信息
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// Set 实例化一致性哈希算法和添加节点
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil) // 给httpPool设置哈希实例
	p.peers.Add(peers...)                              // 增加节点
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers { // 为每个节点(地址)绑定一个http客户端
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath} // httpGetter的地址为节点名称加通用前缀
	}
}

// PickPeer 包装一致性哈希的Get方法，根据key选择具体的节点并返回节点对应的客户端
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self { // 从哈希环中选出key对应的节点，确保其不为空和自身节点
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true // 返回该节点的httpGetter
	}
	return nil, false
}

var _ PeerGetter = (*httpGetter)(nil) // 提供编译器静态检查，判断httpGetter是否实现了PeerGetter这个接口（类型断言）

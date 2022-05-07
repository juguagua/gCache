package main

import (
	"flag"
	"fmt"
	"gCache/gcache"
	"log"
	"net/http"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func main() {
	var port int
	var api bool
	flag.IntVar(&port, "port", 8001, "gCache server port")
	flag.BoolVar(&api, "api", false, "start a api server ?")
	flag.Parse()

	apiAddr := "http://localhost:9999"
	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}
	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	g := createGroup()
	if api {
		go startAPIServer(apiAddr, g)
	}
	startCacheServe(addrMap[port], addrs, g)
}

func createGroup() *gcache.Group {
	return gcache.NewGroup("score", gcache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("search key ", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		},
	), 2<<10)
}

// 启动缓存服务器，创建httpPool，添加节点信息，注册到group中，启动http服务
func startCacheServe(addr string, addrs []string, g *gcache.Group) {
	peers := gcache.NewHTTPPool(addr) // 创建HTTPPool，本身节点地址为addr
	peers.Set(addrs...)               // 给peers（HTTPPool）实例化哈希结构，并添加哈希节点
	g.RegisterPeers(peers)            // 将peers（HTTPPool）注入给group缓存空间
	log.Println("geecache is running at", addr)
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}

// 启动API服务，与用户进行交互
func startAPIServer(apiAddr string, g *gcache.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key") // 处理函数进行参数处理
			view, err := g.Get(key)         // 请求缓存值
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice()) // 将结果写入响应的body中返回
		}))
	log.Println("fontend server is running at", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}

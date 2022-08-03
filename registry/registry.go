package registry

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	etcd "go.etcd.io/etcd/client/v3"
	"sync"
	"time"
)

const (
	// 续约间隔，单位秒
	keepAliveTTL = 10
	// 事件通道缓冲区大小
	eventChanSize = 10
)

// Event 服务变化事件
type Event struct {
	AddAddr    string
	DeleteAddr string
}

// Registry 名字服务
type Registry struct {
	endpoints []string // etcd的多个节点服务地址
	mu        sync.Mutex
	client    *etcd.Client
	prefix    string // etcd名字服务key前缀
}

func New(prefix string, endpoints []string) (*Registry, error) {
	client, err := etcd.New(etcd.Config{
		Endpoints:   endpoints,       // etcd的多个节点服务地址
		DialTimeout: 5 * time.Second, // 创建client的首次连接超时时间
	})
	if err != nil {
		return nil, err
	}
	return &Registry{
		endpoints: endpoints,
		client:    client,
		prefix:    prefix,
	}, nil
}

// Register 注册服务
func (r *Registry) Register(ctx context.Context, addr string) error {
	kv := etcd.NewKV(r.client)                   // 通过new kv获取kv接口的实现，可通过kv操作etcd中的数据
	lease := etcd.NewLease(r.client)             // 通过new lease获取Lease对象
	grant, err := lease.Grant(ctx, keepAliveTTL) // 创建一个续约时间为10s的租约
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s%s", r.prefix, addr)
	if _, err := kv.Put(ctx, key, addr, etcd.WithLease(grant.ID)); err != nil {
		return err
	}
	ch, err := lease.KeepAlive(ctx, grant.ID)
	if err != nil {
		return err
	}
	go func() {
		for range ch {
		}
	}()
	return nil
}

// GetAddrs 获取节点地址列表
func (r *Registry) GetAddrs(ctx context.Context) ([]string, error) {
	kv := etcd.NewKV(r.client)
	resp, err := kv.Get(ctx, r.prefix, etcd.WithPrefix())
	if err != nil {
		return nil, err
	}
	addrs := make([]string, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		addrs[i] = string(kv.Value)
	}
	return addrs, nil
}

// Watch 发现服务
func (r *Registry) Watch(ctx context.Context) <-chan Event {
	watcher := etcd.NewWatcher(r.client)
	watchChan := watcher.Watch(ctx, r.prefix, etcd.WithPrefix())
	ch := make(chan Event, eventChanSize)
	go func() {
		for watchRsp := range watchChan {
			for _, event := range watchRsp.Events {
				switch event.Type {
				case mvccpb.PUT:
					ch <- Event{AddAddr: string(event.Kv.Value)}
				case mvccpb.DELETE:
					ch <- Event{DeleteAddr: string(event.Kv.Key[len(r.prefix):])}
				}
			}
		}
		close(ch)
	}()
	return ch
}

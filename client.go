package gcache

import (
	"context"
	"fmt"
	pb "github.com/juguagua/gCache/gcachepb"
	"github.com/juguagua/gCache/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

// client 模块实现cache访问其他远程节点 从而获取缓存的能力

type client struct {
	name string // 服务名称 gcache/ip:addr
}

// Fetch 从remote peer获取对应缓存值
func (c *client) Fetch(group string, key string) (ByteView, error) {
	// 创建一个etcd client
	cli, err := clientv3.New(defaultEtcdConfig)
	if err != nil {
		return ByteView{}, err
	}
	defer cli.Close()
	// 发现服务 取得与服务的连接
	conn, err := registry.EtcdDial(cli, c.name)
	if err != nil {
		return ByteView{}, err
	}
	defer conn.Close()
	grpcClient := pb.NewGroupCacheClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := grpcClient.Get(ctx, &pb.GetRequest{
		Group: group,
		Key:   key,
	})
	if err != nil {
		return ByteView{}, fmt.Errorf("could not get %s/%s from peer %s", group, key, c.name)
	}

	var expire time.Time
	if resp.Expire != 0 {
		expire = time.Unix(resp.Expire/int64(time.Second), resp.Expire%int64(time.Second))
		if time.Now().After(expire) {
			return ByteView{}, fmt.Errorf("peer returned expired value")
		}
	}

	return ByteView{resp.Value, expire}, nil
}

func NewClient(service string) *client {
	return &client{name: service}
}

// 测试Client是否实现了Fetcher接口
var _ Fetcher = (*client)(nil)

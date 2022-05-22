package gcache

import pb "gCache/gcache/gcachepb/gcachepb"

// PeerPicker 节点选择接口
type PeerPicker interface {
	// PickPeer 用于根据传入的key选择相应的PeerGetter节点
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 相当于一个客户端，每个客户端对象需要实现这个接口，通过客户端来访问远程节点以获得数据
type PeerGetter interface {
	// Get 用于从对应的group中查找缓存值
	Get(in *pb.Request, out *pb.Response) error
}

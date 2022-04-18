package gcache

// PeerPicker 是用来定位的接口
type PeerPicker interface {
	// PickPeers 用于根据传入的key选择合适的PeerGetter节点
	PickPeers(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 相当于一个客户端，每个客户端对象需要实现这个接口
type PeerGetter interface {
	// Get 用于从对应的group中查找缓存值
	Get(group string, key string) ([]byte, error)
}

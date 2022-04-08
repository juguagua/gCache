package gcache


type PeerPicker interface {
	// PickPeers 用于根据传入的key选择合适的PeerGetter节点
	PickPeers(key string) (peer PeerGetter, ok bool)
}

type PeerGetter interface {
	// Get 用于从对应的group中查找缓存值
	Get(group string, key string) ([]byte, error)
}

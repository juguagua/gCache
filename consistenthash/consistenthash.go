package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// consistent hash 模块负责实现一致性哈希
// 用于确定key与peer之间的映射

// Hash 映射bytes到uint32，用于散列键
type Hash func(data []byte) uint32

// Consistence 包含所有被散列的键
type Consistence struct {
	hash     Hash           // 哈希函数依赖
	replicas int            // 虚拟节点倍数
	ring     []int          // 哈希环
	hashMap  map[int]string // 虚拟节点hash到真实节点名称的映射
}

// New 创建一个一致性哈希结构
func New(replicas int, fn Hash) *Consistence {
	m := &Consistence{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil { // 默认散列函数为crc32
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Register 将各个peer注册到哈希环上
func (c *Consistence) Register(peersName ...string) {
	for _, peerName := range peersName {
		for i := 0; i < c.replicas; i++ { // 对于每一个节点都创建多个虚拟节点加入到哈希环中
			hashValue := int(c.hash([]byte(strconv.Itoa(i) + peerName))) // 每个虚拟节点的哈希值为其编号加节点名称进行散列
			c.ring = append(c.ring, hashValue)
			c.hashMap[hashValue] = peerName
		}
	}
	sort.Ints(c.ring) // 添加完后进行排序，打乱节点
}

// Delete 从一致性哈希删除节点
func (m *Consistence) Delete(keys ...string) {
	for _, key := range keys { // 删除指定的节点
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			delete(m.hashMap, hash)
		}
	}
	newKeys := make([]int, 0, len(m.hashMap)) // 重建哈希环
	for key := range m.hashMap {
		newKeys = append(newKeys, key)
	}
	m.ring = newKeys
	sort.Ints(m.ring)
}

// GetPeer 计算key应缓存到的peer
func (c *Consistence) GetPeer(key string) string {
	if len(c.ring) == 0 {
		return ""
	}
	hashValue := int(c.hash([]byte(key)))
	idx := sort.Search(len(c.ring), func(i int) bool { // 二分查找第一个比它哈希值大的节点
		return c.ring[i] >= hashValue
	})
	return c.hashMap[c.ring[idx%len(c.ring)]]
}

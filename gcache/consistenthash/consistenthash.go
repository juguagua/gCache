package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash 哈希算法，采用依赖注入的方式，可自定义
type Hash func(data []byte) uint32

type Map struct {
	hash     Hash           // 哈希函数
	replicas int            // 虚拟节点倍数
	keys     []int          // 哈希环
	hashMap  map[int]string //虚拟节点和真实节点的映射，虚拟节点的哈希值来映射真实节点的名称
}

// New Map的构造方法，允许自定义哈希函数和虚拟节点倍数
func New(replicas int, fn Hash) *Map {
	m := &Map{
		hash:     fn,
		replicas: replicas,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE // 默认哈希算法为crc32
	}
	return m
}

// Add 函数允许传入0或多个真实节点的名称
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ { // 对每一个真实节点创建指定个虚拟节点
			hash := int(m.hash([]byte(strconv.Itoa(i) + key))) // 虚拟节点名称为编号加真实节点名
			m.keys = append(m.keys, hash)                      // 将虚拟节点的哈希值加到哈希环上
			m.hashMap[hash] = key                              // 将虚拟节点和真实节点映射起来
		}
	}
	sort.Ints(m.keys) // 给哈希环上的节点排序，方便后续再插入节点
}

// Get 为请求访问资源的key选择节点
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	return m.hashMap[m.keys[idx%len(m.keys)]]
}

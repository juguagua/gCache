package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash 哈希算法，采用依赖注入的方式，可自定义，默认为 crc32.ChecksumIEEE 算法
type Hash func(data []byte) uint32

type Map struct {
	hash     Hash           // 哈希函数
	replicas int            // 虚拟节点倍数
	keys     []int          // 哈希环
	hashMap  map[int]string // 虚拟节点和真实节点的映射，虚拟节点的名称（哈希值）来映射真实节点的名称
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

// Add 添加真实节点 （允许传入0个或多个真实节点的名称）
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ { // 根据虚拟节点倍数对每一个真实节点创建虚拟节点
			hash := int(m.hash([]byte(strconv.Itoa(i) + key))) // 虚拟节点名称（哈希值）为 编号 + 真实节点名
			m.keys = append(m.keys, hash)                      // 将虚拟节点（哈希值）加到哈希环上
			m.hashMap[hash] = key                              // 将虚拟节点和真实节点映射起来
		}
	}
	sort.Ints(m.keys) // 给哈希环上的节点排序，维护好节点方便后续更新
}

// Get 为请求的key选择节点
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))                   // 计算key的哈希值
	idx := sort.Search(len(m.keys), func(i int) bool { // 通过标准库中的二分查找找到大于等于key的哈希值的最近的节点编号
		return m.keys[i] >= hash
	})
	return m.hashMap[m.keys[idx%len(m.keys)]] // 找到目标节点映射的真实节点，如果 idx == len(m.keys)，说明应选择 m.keys[0]，因为 m.keys 是一个环状结构，所以用取余数的方式来处理这种情况。
}

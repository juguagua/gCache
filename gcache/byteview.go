package gcache

// A ByteView holds an immutable view of bytes
type ByteView struct { // 缓存值的封装结构
	b []byte // byteView只有一个成员变量[]byte, 其会存储真实的缓存值
}

// 实现lru中Value接口的Len()方法，即实现Value接口
func (v ByteView) Len() int {
	return len(v.b)
}

// 因为b是一个切片类型，直接返回其值可能被外部程序修改，因此外部程序需要时返回其的一个拷贝
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// 添加String方法方便将缓存值转为字符串
func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

package gcache

import "time"

/*
	ByteView 一个不可变的字节数组视图，定义读取缓存结果

	实际上 byteview 只是简单的封装了byte slice，让其只读。
	试想一下，直接返回slice，在golang里，一切参数按值传递。
	slice底层只是一个struct，记录着ptr/len/cap，相当于
	复制了一份这三者的值。因此[]byte底层指向同一片内存区域
	我们的缓存底层是存储在LRU的双向链表的Element里，因此
	可以被恶意修改。因此需要将slice封装成只读的ByteView
*/
type ByteView struct {
	b      []byte
	expire time.Time // 过期时间
}

func NewByteView(b []byte, expire time.Time) ByteView {
	return ByteView{
		b:      b,
		expire: expire,
	}
}

func (v ByteView) Expire() time.Time {
	return v.expire
}

func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice 返回一份[]byte的副本（深拷贝）
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

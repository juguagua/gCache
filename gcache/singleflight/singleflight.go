package singleflight

import "sync"

type call struct { // 表示正在进行中，或已经结束的请求
	wg  sync.WaitGroup // 避免重入
	val interface{}
	err error
}

type Group struct { // 管理不同key的请求（call）
	mu sync.Mutex // 保护变量 m 不被并发读写
	m  map[string]*call
}

// 针对相同的 key，无论 Do 被调用多少次，函数 fn 都只会被调用一次，等待 fn 调用结束了，返回返回值或错误
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock() // 加锁，防止对m并发读写
	if g.m == nil {
		g.m = make(map[string]*call) // 延迟初始化
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()         // 如果请求正在进行中，则等待（阻塞）
		return c.val, c.err // 请求结束后返回结果
	}
	c := new(call)
	c.wg.Add(1)   // 发起请求前加锁（锁加一）
	g.m[key] = c  // 将call添加到g.m中，表示该key已经有请求在处理了，其他的要等待
	g.mu.Unlock() // 对m读写完成后解锁

	c.val, c.err = fn() // 调用fn， 发起请求
	c.wg.Done()         // 请求结束（锁减一）

	g.mu.Lock()      // 对g.m操作，加锁
	delete(g.m, key) // 更新g.m
	g.mu.Unlock()

	return c.val, c.err
}

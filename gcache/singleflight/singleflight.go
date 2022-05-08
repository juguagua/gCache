package singleflight

import "sync"

type call struct { // 表示正在进行中，或已经结束的请求
	wg  sync.WaitGroup // 避免重入
	val interface{}
}

type Group struct { // 管理不同key的请求（call）
	mu sync.Mutex
	m  map[string]*call
}

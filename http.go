package gCache

const defaultBasePath = "/_gcache/"

type HTTPPool struct {
	self     string // 用来记录自己的地址，包括主机名/IP/端口号
	basePath string // 节点间通讯地址的前缀
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

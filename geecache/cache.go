package geecache

import (
	"sync"

	"geektutu/geecache/lru"
)

type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		// 什么时候初始化c.cacheBytes？
		// 使用composite literal也是一种有效的初始化，并非一定要定义类型相关的New函数。
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (ByteView, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru != nil {
		if v, ok := c.lru.Get(key); ok {
			return v.(ByteView), true
		}
	}
	return ByteView{}, false
}

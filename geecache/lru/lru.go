package lru

import "container/list"

// 为什么我们要暴露lru中的相关函数呢？是为了让其他包也能利用吗？
type Cache struct {
	maxBytes  int64
	nBytes    int64
	ll        *list.List
	cache     map[string]*list.Element
	OnEvicted func(key string, value Value)
}

type Entry struct {
	key   string
	value Value
}

type Value interface {
	Len() int
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

func (c *Cache) Get(key string) (Value, bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*Entry)
		return kv.value, true
	}
	return nil, false
}

func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*Entry)
		delete(c.cache, kv.key)
		c.nBytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

func (c *Cache) Add(key string, value Value) {
	// if key exist, then update value, otherwise insert
	if v, ok := c.Get(key); ok {
		kv := c.cache[key].Value.(*Entry)
		kv.value = value
		c.nBytes += int64(kv.value.Len()) - int64(v.Len())
	} else { // key non-exist， then先插入
		kv := &Entry{key, value}
		ele := c.ll.PushFront(kv)
		c.cache[key] = ele
		c.nBytes += int64(len(kv.key)) + int64(kv.value.Len())
	}
	// 最后再检查更新
	// c.maxBytes为0表示不限制大小
	if c.maxBytes != 0 && c.nBytes > c.maxBytes {
		c.RemoveOldest()
	}
}

func (c *Cache) Len() int {
	return c.ll.Len()
}

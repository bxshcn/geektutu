package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

// 抽象对象就是关注其属性和行为
// 一致性hash这个对象的属性，包括虚拟扩充节点，其上的keys，以及hashmap将虚拟节点映射到物理节点上。
// 其行为主要是添加物理节点，或者删除物理节点，或者根据key找到对应的物理节点。
// replicas是虚拟节点倍数，用于减轻数据倾斜
// keys是所有节点的hash值，表示一个hash环
// hashmap用于将多个虚拟keys映射为实际的节点名
type Map struct {
	hash     Hash
	replicas int
	keys     []int
	hashmap  map[int]string
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashmap:  make(map[int]string),
	}

	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Add向Map对象中添加物理节点，以及虚拟节点
// 填充hash环keys,以及hashmap
func (m *Map) Add(nodes ...string) {
	for _, node := range nodes {
		for i := 0; i < m.replicas; i++ {
			vnode := strconv.Itoa(i) + node
			vkey := int(m.hash([]byte(vnode)))
			m.keys = append(m.keys, vkey)
			// 将相关vkey映射为同一个物理节点
			m.hashmap[vkey] = node
		}
	}
	sort.Ints(m.keys)
}

/*
func (m *Map) Delete(nodes ...string) {
	for _, node := range nodes {
		for k, v := range m.hashmap {
			if v == node {
				delete(m.hashmap, k)
				deleteSlice(m.keys, k)
			}
		}
	}
}

// 注意s是有序的
func deleteSlice(s []int, k int) {
	p := sort.Search(len(s), func(i int) bool {
		return s[i] >= k
	})
	s = append(s[:p], s[p+1:]...)
} */

// 根据键值（类型为key），将其映射到特定的物理节点
func (m *Map) Get(key string) string {
	// 我们首先计算key的hash值（string->int)，然后根据int得到hash环上的虚拟节点对应的key
	// 然后根据这个虚拟key，以及m.hasmap得到对应的物理node
	// k为key对应到hash环上的的位置
	k := int(m.hash([]byte(key)))

	n := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= k
	})
	// 注意n有可能等于len(m.keys)，也就是说，k大于环上的所有keys。因此要注意取模！
	return m.hashmap[m.keys[n%len(m.keys)]]
}

package geecache

import pb "geektutu/geecache/geecachepb"

// 客户端可能首先会根据key找到存放的peer节点，然后从该peer节点中获取数据，但也有可能本地的原因，选择了其他peer节点：
// 对于一个peer节点来说，如果该key已缓存，则直接返回，否则需要先判断该key是本地还是远程节点
// 如果是远程，则发起http请求获取该数据并返回给客户端
// 如果是本地，则调用回调函数获取数据（同时加载到本地缓存），然后返回给客户端。

// 根据key找到一个缓存有该key的节点
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// 一个httppool客户端，用于从特定的缓存（根据key查找到的缓存）获取数据。
/*type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
}*/
type PeerGetter interface {
	Get(*pb.Request, *pb.Response) error
}

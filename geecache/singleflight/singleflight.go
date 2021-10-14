package singleflight

import "sync"

type call struct {
	// 多个访问同一key的调用，隶属于同一个waitgroup，那么首次调用会wg.Add(1)，并在结束后wg.Done()，
	// 而其他并发调用会直接调用wg.Wait()，表示等待首次调用结束。
	wg sync.WaitGroup
	// call的返回值，多个并发calls共享同一个返回值，一旦有一个发起调用，那么其他的就等待其结束后，返回这里的返回值即可
	val interface{}
	err error
}

// 这里命名为Group，是一个网络层面的Group抽象，其语义与geecache.Group类似：你可以从Group中获取缓存的数据
// key对应m的key，*call对应返回值。
// 但它相当于是一个限制（针对同一个key）进行并发调用访问的工具，我们将这个工具内嵌在geecache.Group中
// 按我的理解，这里改名为SingleCall更好理解，我们将SingleCall作为工具内嵌在Group中。
type SingleCall struct {
	mu sync.Mutex
	m  map[string]*call
}

// Do 利用SingleCall这个数据结构中的属性，实现并发网络调用fn的单次执行
func (sc *SingleCall) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// 首先我们要判断sc.m中是否已经有了针对key的调用，由于对m的访问是并发，因此不论读写，我们都要获取锁
	sc.mu.Lock()
	if sc.m == nil {
		sc.m = make(map[string]*call)
	}
	if c, ok := sc.m[key]; ok {
		// 已经发起了调用，我们在当前线程等待其结束即可
		// 我们不用在访问sc.mu了，所以要即时释放unlock
		// 针对同一个key的所有调用线程，都隶属于同一个wg：sc.m[key].wg
		sc.mu.Unlock()
		c.wg.Wait()
		// 等待结束后，直接返回对应call的返回值即可
		return c.val, c.err
	}
	// 否则我们要发起真正的调用
	// 首先我们要修改sc.m，向其中添加对应call，然后即时释放锁
	// 随后发起真正的调用：
	// 1. 先将call.Add(1)
	// 2. 然后发起调用fn()
	// 3. 最后call.Done()

	c := &call{}
	// 我们不是以发起fn()调用视为发起调用，而是将修改sc.m[key]这个动作视为发起调用，
	// 因为一旦我们修改了这个值，其他线程就必须等待。
	// 所以我们要将下面fn()的调用移到这里。
	c.wg.Add(1)
	sc.m[key] = c
	sc.mu.Unlock()

	//c.wg.Add(1)
	c.val, c.err = fn()
	c.wg.Done()

	// 那么我们什么时候删除这个调用中的key呢？先返回然后再释放（放在defer中），或者直接释放即可。
	sc.mu.Lock()
	delete(sc.m, key)
	sc.mu.Unlock()

	return c.val, c.err
}

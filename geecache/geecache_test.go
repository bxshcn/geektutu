package geecache

import (
	"fmt"
	"log"
	"reflect"
	"testing"
)

func anonymousFunc(key string) ([]byte, error) {
	return []byte(key), nil
}

func TestGetterFunc(t *testing.T) {
	// 利用实现指定接口的函数类型修饰相关函数，实现类型转换。
	aGetter := GetterFunc(anonymousFunc)
	bytes, _ := aGetter.Get("hello")
	if reflect.DeepEqual(bytes, []byte("hello")) == false {
		t.Fatalf("want %v, but get %v", []byte("hello"), bytes)
	}
}

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func TestGet(t *testing.T) {
	gee := NewGroup("scores", 2<<10, GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
	for k := range db {
		// 第一次未命中，调用回调函数从db中获取
		if _, err := gee.Get(k); err != nil {
			log.Fatal("failed to get value")
		}
		// 第二次每次都命中
		if _, err := gee.Get(k); err != nil {
			log.Fatalf("cache %s miss", k)
		}
	}
	// 访问无法获取的数据，会失败返回错误
	if view, err := gee.Get("unkown"); err != nil {
		log.Printf("we should not get 'unknown' and what we get is 0: %v\n", view)
	}
}

package session

import (
	"geektutu/geeorm/log"
	"reflect"
)

const (
	BeforeQuery  = "BeforeQuery"
	AfterQuery   = "AfterQuery"
	BeforeUpdate = "BeforeUpdate"
	AfterUpdate  = "AfterUpdate"
	BeforeDelete = "BeforeDelete"
	AfterDelete  = "AfterDelete"
	BeforeInsert = "BeforeInsert"
	AfterInsert  = "AfterInsert"
)

// CallMethod调用hook方法：s.CallMethod(AfterUpdate, nil)
// 注意这个被调用的方法是hook方法，只与具体的对象类型有关，而与具体的记录无关。
// 所有hook方法都以*session为参数，以error作为输出
func (s *Session) CallMethod(method string, value interface{}) {
	//if fm, ok := s.RefTable().Model.MethodByName(method); ok {}
	// 这里使用New新造一个表对应的go对象，再根据MethodByName得到方法，是否可行呢？
	fm := reflect.New(s.RefTable().Model).Elem().MethodByName(method)

	if value != nil {
		fm = reflect.ValueOf(value).MethodByName(method)
	}
	// param是干什么用的？
	param := []reflect.Value{reflect.ValueOf(s)}
	if fm.IsValid() {
		if v := fm.Call(param); len(v) > 0 {
			if err, ok := v[0].Interface().(error); ok {
				log.Error(err)
			}
		}
	}
	//return
}

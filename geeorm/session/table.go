package session

import (
	"errors"
	"fmt"
	"geektutu/geeorm/log"
	"geektutu/geeorm/schema"
	"reflect"
	"strings"
)

// Model是一个动词，根据一个go领域的对象，得到一个数据库领域的Schema对象（隶属于Session的字段refTable）
// 要想从一个领域A映射到另外一个领域B，我们一般都是在A领域表示B领域的对象，比如用A程序领域的Schema对象表示
// B数据库领域的表；再比如用A类型领域的reflect.Type和reflect.Value对象来表示B泛型领域的类型和值。
// 籍由refelct，我们可以更精确的描述schema和数据库表的关系：即用A类型领域的Schema对象来表示B数据库领域的表
// 换句话说，所谓面向对象思维，就是弄清楚具体领域的对象的特征并用类型对象来表示
//
// 另外，这个返回也是Session，因此这个函数支持串联。比如s.Model(V).xxx
func (s *Session) Model(value interface{}) *Session {
	//if s.refTable == nil || reflect.TypeOf(value) != reflect.TypeOf(s.refTable.Model) {
	if s.refTable == nil || reflect.TypeOf(value) != s.refTable.Model {
		s.refTable = schema.Parse(value, s.dialect)
	}
	return s
}

func (s *Session) RefTable() *schema.Schema {
	if s.refTable != nil {
		return s.refTable
	}
	log.Error("model is not set")
	return nil
}

// 注意s中已经内涵了refTable这个Schema，因此我们相当于已知了table的meta信息
// 据此组织成一个table的创建语句是一件很简单的事。
func (s *Session) CreateTable() error {
	table := s.RefTable()
	if table == nil {
		return errors.New("create table error")
	}
	var columns []string
	for _, field := range table.Fields {
		columns = append(columns, fmt.Sprintf("%s %s %s", field.Name, field.Type, field.Tag))
	}
	desc := strings.Join(columns, ",")

	_, err := s.Raw(fmt.Sprintf("create table %s (%s);", table.Name, desc)).Exec()

	return err
}

func (s *Session) DropTable() error {
	_, err := s.Raw(fmt.Sprintf("drop table if exists %s;", s.RefTable().Name)).Exec()
	return err
}

// 显然，dialect的TableExistSQL完全是为HasTable而创建的工具接口
func (s *Session) HasTable() bool {
	sql, values := s.dialect.TableExistSQL(s.RefTable().Name)
	row := s.Raw(sql, values...).QueryRow()
	var tmp string
	_ = row.Scan(&tmp)
	return tmp == s.RefTable().Name

}

package session

import (
	"database/sql"
	"geektutu/geeorm/clause"
	"geektutu/geeorm/dialect"
	"geektutu/geeorm/schema"
	"strings"
)

// db、sql、sqlVars支撑了原始sql语句的执行
//
// 为了支持基于程序对象的操作，需要先将程序对象映射为相关中间对象, 再基于这个中间对象进行实际操作。
// 以表的增删操作为例，我们需要先将程序对象（比如User结构体）转换为中间对象Schema（借助Dialect），
// 最后将中间对象转换为原始的sql语句并执行。
type Session struct {
	db      *sql.DB
	sql     strings.Builder
	sqlVars []interface{}
	// 支持DDL操作
	// 基于dialect包实现对象到表schema的映射, 注意使用的是dialect包中的全局变量DialectsMap，因此不用指针
	dialect dialect.Dialect
	// session只关联了一张表，因此这个实现是非常简单和受限的。
	// 如果我们希望实现更具有一般性，比如多表关联，那么下面就可能是[]*schema.Schema了。
	refTable *schema.Schema

	// 支持insert等DML具体操作
	clause clause.Clause
}

func New(db *sql.DB, dialect dialect.Dialect) *Session {
	return &Session{db: db, dialect: dialect}
}

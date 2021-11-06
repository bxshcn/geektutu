package clause

/*
我们先创建各个sql的子句，最后执行sql句子。
*/

import "strings"

type Type int

const (
	INSERT Type = iota
	VALUES
	SELECT
	LIMIT
	WHERE
	ORDERBY
	//
	UPDATE
	DELETE
	COUNT
)

// Clause是一个记录有一个sql语句的抽象对象
// 它包含所有的子句，以及子句对应的参数。由于每个子句的结构不同，因此参数也不一样。比如：
// SELECT column,.. FROM tablename，并不涉及参数
// INSERT INTO tablename (column, ) , 与SELECT语句类似，也不涉及参数
// LIMIT ?，则需要一个整数参数;WHERE子句与其类似，WHERE column = ?，需要一个参数
type Clause struct {
	sql     map[Type]string
	sqlVars map[Type][]interface{}
}

func (c *Clause) Set(typ Type, vars ...interface{}) {
	if c.sql == nil {
		c.sql = make(map[Type]string)
		c.sqlVars = make(map[Type][]interface{})
	}
	sql, vars := generators[typ](vars...)
	c.sql[typ] = sql
	c.sqlVars[typ] = vars
}

//
func (c *Clause) Build(typs ...Type) (string, []interface{}) {
	var sqls []string
	var vars []interface{}

	for _, typ := range typs {
		if sql, ok := c.sql[typ]; ok {
			sqls = append(sqls, sql)
			vars = append(vars, c.sqlVars[typ]...)
		}
	}

	return strings.Join(sqls, " "), vars
}

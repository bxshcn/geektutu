package session

import (
	"errors"
	"geektutu/geeorm/clause"
	"reflect"
)

// 最终的用户接口，Insert参数为程序对象
// 插入的动作为Insert into tablename (columns, ) values (?, ), (?, ); args
// 前面的insert into 这个已经通过私有的clause子句函数得到，后面的args使用schema.RecordValues可以得到
// 参数values是否能涉及多种对象的值呢？如果涉及多种对象，我们就要构建多个sql，但一个session只对应一个sql
// 结合clause的限制，我们只能操作一张表，因此values只能属于同一类对象。
func (s *Session) Insert(values ...interface{}) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}

	recordValues := make([]interface{}, 0)
	s.CallMethod(BeforeInsert, values[0]) // 针对所有输入对象进行插入前操作, 修改所得
	table := s.Model(values[0]).RefTable()
	s.clause.Set(clause.INSERT, table.Name, table.FieldNames)
	recordValues = append(recordValues, table.RecordValues(values[0]))

	for i := 1; i < len(values); i++ {
		s.CallMethod(BeforeInsert, values[i]) //
		recordValues = append(recordValues, table.RecordValues(values[i]))
	}

	s.clause.Set(clause.VALUES, recordValues...)

	sql, args := s.clause.Build(clause.INSERT, clause.VALUES)

	result, err := s.Raw(sql, args...).Exec()
	if err != nil {
		return 0, err
	}

	s.CallMethod(AfterInsert, nil) // 做一些扫尾工作，无关具体记录或对象

	return result.RowsAffected()

}

/*
s := geeorm.NewEngine("sqlite3", "gee.db").NewSession()
var users []User
s.Find(&users);
*/
func (s *Session) Find(values interface{}) error {
	s.CallMethod(BeforeQuery, nil)

	destSlice := reflect.Indirect(reflect.ValueOf(values))
	destType := destSlice.Type().Elem()
	table := s.Model(reflect.New(destType).Elem().Interface()).RefTable()

	s.clause.Set(clause.SELECT, table.Name, table.FieldNames)
	sql, vars := s.clause.Build(clause.SELECT, clause.WHERE, clause.ORDERBY, clause.LIMIT)
	rows, err := s.Raw(sql, vars...).QueryRows()
	if err != nil {
		return err
	}

	for rows.Next() {
		dest := reflect.New(destType).Elem()
		var values []interface{}
		for _, name := range table.FieldNames {
			values = append(values, dest.FieldByName(name).Addr().Interface())
		}
		if err := rows.Scan(values...); err != nil {
			return err
		}
		s.CallMethod(AfterQuery, dest.Addr().Interface()) // 获取一行之后，对其进行操作
		destSlice.Set(reflect.Append(destSlice, dest))
	}
	return rows.Close()
}

// support map[string]interface{}
// also support kv list: "Name", "Tom", "Age", 18, ....
func (s *Session) Update(kv ...interface{}) (int64, error) {
	s.CallMethod(BeforeUpdate, nil) //
	m, ok := kv[0].(map[string]interface{})
	if !ok {
		m = make(map[string]interface{})
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
	}
	s.clause.Set(clause.UPDATE, s.RefTable().Name, m)
	sql, vars := s.clause.Build(clause.UPDATE, clause.WHERE)
	result, err := s.Raw(sql, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.CallMethod(AfterUpdate, nil) //
	return result.RowsAffected()
}

// Delete records with where clause
func (s *Session) Delete() (int64, error) {
	s.CallMethod(BeforeDelete, nil) //
	s.clause.Set(clause.DELETE, s.RefTable().Name)
	sql, vars := s.clause.Build(clause.DELETE, clause.WHERE)
	result, err := s.Raw(sql, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.CallMethod(AfterDelete, nil) //
	return result.RowsAffected()
}

// Count records with where clause
func (s *Session) Count() (int64, error) {
	s.clause.Set(clause.COUNT, s.RefTable().Name)
	sql, vars := s.clause.Build(clause.COUNT, clause.WHERE)
	row := s.Raw(sql, vars...).QueryRow()
	var tmp int64
	if err := row.Scan(&tmp); err != nil {
		return 0, err
	}
	return tmp, nil
}

// Limit adds limit condition to clause
func (s *Session) Limit(num int) *Session {
	s.clause.Set(clause.LIMIT, num)
	return s
}

// Where adds limit condition to clause
func (s *Session) Where(desc string, args ...interface{}) *Session {
	var vars []interface{}
	s.clause.Set(clause.WHERE, append(append(vars, desc), args...)...)
	return s
}

// OrderBy adds order by condition to clause
func (s *Session) OrderBy(desc string) *Session {
	s.clause.Set(clause.ORDERBY, desc)
	return s
}

func (s *Session) First(value interface{}) error {
	dest := reflect.Indirect(reflect.ValueOf(value))
	destSlice := reflect.New(reflect.SliceOf(dest.Type())).Elem()
	if err := s.Limit(1).Find(destSlice.Addr().Interface()); err != nil {
		return err
	}
	if destSlice.Len() == 0 {
		return errors.New("NOT FOUND")
	}
	dest.Set(destSlice.Index(0))
	return nil
}

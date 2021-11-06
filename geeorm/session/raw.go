package session

import (
	"database/sql"
	"geektutu/geeorm/clause"
	"geektutu/geeorm/log"
)

func (s *Session) Clear() {
	s.sql.Reset()
	s.sqlVars = nil
	s.clause = clause.Clause{}
}

/*func (s *Session) DB() *sql.DB {
	return s.db
} */
func (s *Session) DB() CommonDB {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

// 改变session中的sql和sqlVars
func (s *Session) Raw(sql string, sqlVars ...interface{}) *Session {
	s.sql.WriteString(sql)
	s.sql.WriteString(" ") // why we need this?

	s.sqlVars = append(s.sqlVars, sqlVars...)
	return s
}

func (s *Session) Exec() (sql.Result, error) {
	defer s.Clear()

	log.Info(s.sql.String(), s.sqlVars)
	result, err := s.DB().Exec(s.sql.String(), s.sqlVars...)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return result, nil
}

func (s *Session) QueryRow() *sql.Row {
	defer s.Clear()

	log.Info(s.sql.String(), s.sqlVars)
	return s.DB().QueryRow(s.sql.String(), s.sqlVars...)
}

func (s *Session) QueryRows() (*sql.Rows, error) {
	defer s.Clear()

	log.Info(s.sql.String(), s.sqlVars)

	rows, err := s.DB().Query(s.sql.String(), s.sqlVars...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

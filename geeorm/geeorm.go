package geeorm

import (
	"database/sql"
	"geektutu/geeorm/dialect"
	"geektutu/geeorm/log"
	"geektutu/geeorm/session"
)

type Engine struct {
	db      *sql.DB
	dialect dialect.Dialect
}

func NewEngine(driver, source string) (e *Engine, err error) {
	// Open may just validate its arguments without creating a connection
	// to the database. To verify that the data source name is valid, call Ping.
	// The returned DB is safe for concurrent use by multiple goroutines
	// and maintains its own pool of idle connections. Thus, the Open function
	// should be called just once. It is rarely necessary to close a DB.
	db, err := sql.Open(driver, source)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if err = db.Ping(); err != nil {
		log.Error(err)
	}
	dialect, ok := dialect.GetDialect(driver)
	if !ok {
		log.Errorf("dialect %s Not Found", driver)
		return
	}
	log.Info("connect database successfully.")
	return &Engine{db: db, dialect: dialect}, nil
}

func (engine *Engine) Close() {
	// Close closes the database and prevents new queries from starting.
	// Close then waits for all queries that have started processing
	// on the server to finish.
	// It is rare to Close a DB, as the DB handle is meant to be long-lived
	// and shared between many goroutines.
	if err := engine.db.Close(); err != nil {
		log.Error(err)
	}
	log.Info("Close database connection successfully")
}

func (engine *Engine) NewSession() *session.Session {
	return session.New(engine.db, engine.dialect)
}

package postgres

import (
	"database/sql"
	"sync"
	"sync/atomic"
)

var (
	currentID   = new(int64)
	connections sync.Map // connection id → *sql.DB
)

func nextID() int64 {
	return atomic.AddInt64(currentID, 1)
}

func getConn(id int64) *sql.DB {
	db, ok := connections.Load(id)
	if !ok {
		return nil
	}
	return db.(*sql.DB)
}

func setConn(id int64, db *sql.DB) {
	connections.Store(id, db)
}

func deleteConn(id int64) {
	connections.Delete(id)
}

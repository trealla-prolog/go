package trealla

import (
	"sync"
)

type DB struct {
	mother *prolog
	pool   *sync.Pool
	tx     int
	mu     *sync.RWMutex
	// ch     chan struct{}
}

func NewDB() (*DB, error) {
	pl, err := New()
	if err != nil {
		return nil, err
	}
	db := &DB{
		mother: pl.(*prolog),
		pool:   new(sync.Pool),
		mu:     new(sync.RWMutex),
		// ch:     make(chan struct{}, 25),
	}
	db.pool.New = func() any {
		// db.ch <- struct{}{}
		// defer func() { <-db.ch }()
		ch, err := db.spawn()
		if err != nil {
			panic(err)
		}
		return ch
	}
	for i := 0; i < 10; i++ {
		x, _ := db.spawn()
		db.pool.Put(x)
	}
	return db, nil
}

func (db *DB) WriteTx(tx func(Prolog) error) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	pl := &lockedProlog{prolog: db.mother}
	db.tx++
	err := tx(pl)
	return err
}

func (db *DB) ReadTx(tx func(Prolog) error) error {
	db.mu.RLock()
	defer db.mu.RUnlock()
	child, err := db.child()
	if err != nil {
		return err
	}
	err = tx(child)
	child.done(db)
	return err
}

func (db *DB) child() (*child, error) {
	child := db.pool.Get().(*child)
	if child.tx < db.tx {
		if err := child.prolog.init(db.mother); err != nil {
			return nil, err
		}
		child.tx = db.tx
	}
	return child, nil
}

func (db *DB) Stats() Stats {
	db.mu.RLock()
	defer db.mu.RUnlock()
	child := db.pool.Get().(*child)
	defer child.done(db)
	return child.prolog.stats()
}

type child struct {
	*lockedProlog
	tx int
}

func (c *child) done(db *DB) {
	db.pool.Put(c)
}

func (db *DB) spawn() (*child, error) {
	pl, err := db.mother.clone()
	if err != nil {
		return nil, err
	}
	return &child{
		lockedProlog: &lockedProlog{prolog: pl},
		tx:           db.tx,
	}, nil
}

// func ()

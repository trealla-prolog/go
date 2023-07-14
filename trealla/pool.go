//xxx go:build experiment

package trealla

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

type Pool struct {
	canon    *prolog
	children []*child
	tx       uint
	mu       *sync.RWMutex

	// round robin counter
	rr *uint64

	// options
	size int
	cfg  []Option
}

func NewPool(options ...PoolOption) (*Pool, error) {
	pool := &Pool{
		size: runtime.NumCPU(),
		mu:   new(sync.RWMutex),
		rr:   new(uint64),
	}
	for _, opt := range options {
		if err := opt(pool); err != nil {
			return nil, err
		}
	}
	pl, err := New(pool.cfg...)
	if err != nil {
		return nil, err
	}
	pool.canon = pl.(*prolog)
	pool.children = make([]*child, pool.size)
	for i := range pool.children {
		var err error
		pool.children[i], err = pool.spawn()
		if err != nil {
			return nil, err
		}
	}
	return pool, nil
}

func (pool *Pool) WriteTx(tx func(Prolog) error) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pl := &lockedProlog{prolog: pool.canon}
	defer pl.kill()
	pool.tx++
	err := tx(pl)

	if err == nil {
		for _, child := range pool.children {
			if err := child.become(pool.canon); err != nil {
				return err
			}
			child.tx = pool.tx
		}
	}

	return err
}

func (db *Pool) ReadTx(tx func(Prolog) error) error {
	db.mu.RLock()
	defer db.mu.RUnlock()
	child := db.child()
	child.mu.Lock()
	defer child.mu.Unlock()
	err := tx(&lockedProlog{prolog: child.prolog})
	return err
}

func (db *Pool) child() *child {
	n := atomic.AddUint64(db.rr, 1) % uint64(len(db.children))
	child := db.children[n]
	return child
}

func (db *Pool) Stats() Stats {
	db.mu.RLock()
	defer db.mu.RUnlock()
	n := atomic.LoadUint64(db.rr) % uint64(len(db.children))
	child := db.children[n]
	return child.prolog.Stats()
}

type child struct {
	*prolog
	tx uint
}

func (db *Pool) spawn() (*child, error) {
	pl, err := db.canon.clone()
	if err != nil {
		return nil, err
	}
	return &child{
		prolog: pl,
		tx:     db.tx,
	}, nil
}

type PoolOption func(*Pool) error

func WithPoolSize(instances int) PoolOption {
	return func(p *Pool) error {
		if instances < 1 {
			return fmt.Errorf("trealla: pool size too low: %d", instances)
		}
		p.size = instances
		return nil
	}
}

func WithPoolConfig(options ...Option) PoolOption {
	return func(p *Pool) error {
		p.cfg = append(p.cfg, options...)
		return nil
	}
}

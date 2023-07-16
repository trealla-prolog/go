//xxx go:build experiment

package trealla

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// Pool is a pool of Prolog interpreters that distributes read requests to replicas.
type Pool struct {
	canon    *prolog
	children []*prolog
	tx       uint
	mu       *sync.RWMutex

	// round robin counter
	rr *uint64

	// options
	size int
	cfg  []Option
}

// NewPool creates a new pool with the given options.
// By default, the pool size will match the number of available CPUs.
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
	pool.children = make([]*prolog, pool.size)
	for i := range pool.children {
		var err error
		pool.children[i], err = pool.spawn()
		if err != nil {
			return nil, err
		}
	}
	return pool, nil
}

// WriteTx executes a write transaction against this Pool.
// Use this when modifying the knowledgebase (assert/retract, consulting files, loading modules, and so on).
func (pool *Pool) WriteTx(tx func(Prolog) error) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pl := &lockedProlog{prolog: pool.canon}
	defer pl.kill()
	pool.tx++
	err := tx(pl)

	// Eagerly update the replicas.
	// This seems to be faster than lazily updating them.
	if err == nil {
		for _, child := range pool.children {
			if err := child.become(pool.canon); err != nil {
				return err
			}
		}
	}

	return err
}

// ReadTx executes a read transaction against this Pool.
// Queries in a read transaction must not modify the knowledgebase.
func (db *Pool) ReadTx(tx func(Prolog) error) error {
	db.mu.RLock()
	defer db.mu.RUnlock()
	child := db.child()
	child.mu.Lock()
	defer child.mu.Unlock()
	pl := &lockedProlog{prolog: child}
	defer pl.kill()
	err := tx(pl)
	return err
}

func (db *Pool) child() *prolog {
	n := atomic.AddUint64(db.rr, 1) % uint64(len(db.children))
	child := db.children[n]
	return child
}

func (db *Pool) Stats() Stats {
	db.mu.RLock()
	defer db.mu.RUnlock()
	n := atomic.LoadUint64(db.rr) % uint64(len(db.children))
	child := db.children[n]
	return child.Stats()
}

func (db *Pool) spawn() (*prolog, error) {
	pl, err := db.canon.clone()
	if err != nil {
		return nil, err
	}
	return pl, nil
}

// PoolOption is an option for configuring a Pool.
type PoolOption func(*Pool) error

// WithPoolSize configures the size (number of replicas) of the Pool.
func WithPoolSize(replicas int) PoolOption {
	return func(pool *Pool) error {
		if replicas < 1 {
			return fmt.Errorf("trealla: pool size too low: %d", replicas)
		}
		pool.size = replicas
		return nil
	}
}

// WithPoolPrologOption configures interpreter options for the instances of a Pool.
func WithPoolPrologOption(options ...Option) PoolOption {
	return func(pool *Pool) error {
		pool.cfg = append(pool.cfg, options...)
		return nil
	}
}

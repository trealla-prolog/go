package trealla

import (
	"fmt"
	"log"
	"runtime"
	"sync"
)

// Pool is a pool of Prolog interpreters that distributes read requests to replicas.
type Pool struct {
	canon    *prolog
	children []*prolog
	idle     chan *prolog
	mu       *sync.RWMutex

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
	log.Println("daddy procs", pl.(*prolog).procs)
	pool.canon = pl.(*prolog)
	pool.children = make([]*prolog, pool.size)
	pool.idle = make(chan *prolog, pool.size)
	for i := range pool.children {
		var err error
		pool.children[i], err = pool.spawn()
		if err != nil {
			return nil, err
		}
		pool.idle <- pool.children[i]
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
func (pool *Pool) ReadTx(tx func(Prolog) error) error {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	child := pool.child()
	defer pool.done(child)
	child.mu.Lock()
	defer child.mu.Unlock()
	pl := &lockedProlog{prolog: child}
	defer pl.kill()
	err := tx(pl)
	return err
}

func (pool *Pool) Stats() Stats {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	child := pool.child()
	defer pool.done(child)
	return child.Stats()
}

func (pool *Pool) spawn() (*prolog, error) {
	return pool.canon.clone()
}

func (pool *Pool) child() *prolog {
	return <-pool.idle
}

func (pool *Pool) done(child *prolog) {
	pool.idle <- child
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

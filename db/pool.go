// /home/krylon/go/src/github.com/blicero/raconteur/db/pool.go
// -*- mode: go; coding: utf-8; -*-
// Created on 13. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2021-09-13 23:19:31 krylon>

package db

import (
	"fmt"
	"log"
	"sync"

	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/logdomain"
)

type dblink struct {
	db   *Database
	next *dblink
}

// Pool is a pool of database connections
type Pool struct {
	cnt   int
	log   *log.Logger
	link  *dblink
	lock  sync.RWMutex
	empty *sync.Cond
}

// NewPool creates a Pool of database connections.
// The number of connections to use is given by the
// parameter cnt.
func NewPool(cnt int) (*Pool, error) {
	var (
		err  error
		pool = &Pool{cnt: cnt}
	)

	pool.empty = sync.NewCond(&pool.lock)

	if cnt < 1 {
		return nil, fmt.Errorf(
			"NewPool expects a positive number, you passed %d",
			cnt)
	} else if pool.log, err = common.GetLogger(logdomain.DBPool); err != nil {
		return nil, err
	}

	for i := 0; i < cnt; i++ {
		var link = &dblink{next: pool.link}

		if link.db, err = Open(common.DbPath); err != nil {
			pool.log.Printf("[ERROR] Cannot open database: %s\n",
				err.Error())
			return nil, err
		}

		pool.link = link
	}

	return pool, nil
} // func NewPool(cnt int) (*Pool, error)

// Close closes all open database connections currently in the pool and empties
// the pool. Any connections retrieved from the pool that are in use at the
// time Close is called are unaffected.
func (pool *Pool) Close() error {
	pool.lock.Lock()

	for link := pool.link; link != nil; link = link.next {
		link.db.Close() // nolint: errcheck,gosec
		link.db = nil
	}

	pool.link = nil
	pool.cnt = 0
	pool.lock.Unlock()
	return nil
} // func (pool *Pool) Close() error

// Get returns a DB connection from the pool.
// If the pool is empty, it waits for a connection to be returned.
func (pool *Pool) Get() *Database {
	var link *dblink

	pool.lock.Lock()
	defer pool.lock.Unlock()

WAIT_FOR_LINK:
	if pool.link != nil {
		link = pool.link
		pool.link = link.next
		pool.cnt--

		link.next = nil
		return link.db
	}

	// Wait for it!!!
	pool.empty.Wait()
	goto WAIT_FOR_LINK
} // func (pool *Pool) Get() *DB

// GetNoWait returns a DB connection from the pool.
// If the pool is empty, it creates a new one.
func (pool *Pool) GetNoWait() (*Database, error) {
	var db *Database
	var err error

	pool.lock.Lock()
	defer pool.lock.Unlock()

	if pool.link != nil {
		link := pool.link
		pool.link = link.next
		pool.cnt--
		return link.db, nil
	} else if db, err = Open(common.DbPath); err != nil {
		pool.log.Printf("[ERROR] Error opening new database connection: %s",
			err.Error())
		return nil, err
	}

	return db, nil
} // func (pool *Pool) GetNoWait() *Database

// Put returns a DB connection to the pool.
func (pool *Pool) Put(db *Database) {
	link := &dblink{
		db: db,
	}

	if db.tx != nil {
		pool.log.Println("[INFO] DB has pending transaction, rolling back.")
		if err := db.Rollback(); err != nil {
			pool.log.Printf("[ERROR] Cannot roll back transaction: %s\n",
				err.Error())
		}
	}

	pool.lock.Lock()
	link.next = pool.link
	pool.link = link
	pool.cnt++
	pool.lock.Unlock()
	pool.empty.Signal()
} // func (pool *Pool) Put(db *Database)

// IsEmpty returns true if the pool is currently empty.
func (pool *Pool) IsEmpty() bool {
	pool.lock.RLock()
	var empty = pool.link == nil
	pool.lock.RUnlock()
	return empty
} // func (pool *Pool) IsEmpty() bool

// Local Variables:  //
// compile-command: "go generate && go vet && go build -v -p 4 && mygolint ticker/database && go test -v" //
// End: //

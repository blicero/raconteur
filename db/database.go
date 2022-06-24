// /home/krylon/go/src/github.com/blicero/raconteur/db/database.go
// -*- mode: go; coding: utf-8; -*-
// Created on 08. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-24 21:30:00 krylon>

// Package db provides a wrapper around the actual database connection.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/db/query"
	"github.com/blicero/raconteur/logdomain"
	"github.com/blicero/raconteur/objects"

	_ "github.com/mattn/go-sqlite3" // Import the database driver
)

var (
	openLock sync.Mutex
	idCnt    int64
)

// ErrTxInProgress indicates that an attempt to initiate a transaction failed
// because there is already one in progress.
var ErrTxInProgress = errors.New("A Transaction is already in progress")

// ErrNoTxInProgress indicates that an attempt was made to finish a
// transaction when none was active.
var ErrNoTxInProgress = errors.New("There is no transaction in progress")

// ErrEmptyUpdate indicates that an update operation would not change any
// values.
var ErrEmptyUpdate = errors.New("Update operation does not change any values")

// ErrInvalidValue indicates that one or more parameters passed to a method
// had values that are invalid for that operation.
var ErrInvalidValue = errors.New("Invalid value for parameter")

// ErrObjectNotFound indicates that an Object was not found in the database.
var ErrObjectNotFound = errors.New("object was not found in database")

// ErrInvalidSavepoint is returned when a user of the Database uses an unkown
// (or expired) savepoint name.
var ErrInvalidSavepoint = errors.New("that save point does not exist")

// If a query returns an error and the error text is matched by this regex, we
// consider the error as transient and try again after a short delay.
var retryPat = regexp.MustCompile("(?i)database is (?:locked|busy)")

// worthARetry returns true if an error returned from the database
// is matched by the retryPat regex.
func worthARetry(e error) bool {
	return retryPat.MatchString(e.Error())
} // func worthARetry(e error) bool

// retryDelay is the amount of time we wait before we repeat a database
// operation that failed due to a transient error.
const retryDelay = 25 * time.Millisecond

func waitForRetry() {
	time.Sleep(retryDelay)
} // func waitForRetry()

// Database is the storage backend for managing podcasts audio books.
//
// It is not safe to share a Database instance between goroutines, however
// opening multiple connections to the same Database is safe.
type Database struct {
	id            int64
	db            *sql.DB
	tx            *sql.Tx
	log           *log.Logger
	path          string
	spNameCounter int
	spNameCache   map[string]string
	queries       map[query.ID]*sql.Stmt
}

// Open opens a Database. If the database specified by the path does not exist,
// yet, it is created and initialized.
func Open(path string) (*Database, error) {
	var (
		err      error
		dbExists bool
		db       = &Database{
			path:          path,
			spNameCounter: 1,
			spNameCache:   make(map[string]string),
			queries:       make(map[query.ID]*sql.Stmt),
		}
	)

	openLock.Lock()
	defer openLock.Unlock()
	idCnt++
	db.id = idCnt

	if db.log, err = common.GetLogger(logdomain.Database); err != nil {
		return nil, err
	} else if common.Debug {
		db.log.Printf("[DEBUG] Open database %s\n", path)
	}

	var connstring = fmt.Sprintf("%s?_locking=NORMAL&_journal=WAL&_fk=1&recursive_triggers=0",
		path)

	if dbExists, err = krylib.Fexists(path); err != nil {
		db.log.Printf("[ERROR] Failed to check if %s already exists: %s\n",
			path,
			err.Error())
		return nil, err
	} else if db.db, err = sql.Open("sqlite3", connstring); err != nil {
		db.log.Printf("[ERROR] Failed to open %s: %s\n",
			path,
			err.Error())
		return nil, err
	}

	if !dbExists {
		if err = db.initialize(); err != nil {
			var e2 error
			if e2 = db.db.Close(); e2 != nil {
				db.log.Printf("[CRITICAL] Failed to close database: %s\n",
					e2.Error())
				return nil, e2
			} else if e2 = os.Remove(path); e2 != nil {
				db.log.Printf("[CRITICAL] Failed to remove database file %s: %s\n",
					db.path,
					e2.Error())
			}
			return nil, err
		}
		db.log.Printf("[INFO] Database at %s has been initialized\n",
			path)
	}

	return db, nil
} // func Open(path string) (*Database, error)

func (db *Database) initialize() error {
	var err error
	var tx *sql.Tx

	if common.Debug {
		db.log.Printf("[DEBUG] Initialize fresh database at %s\n",
			db.path)
	}

	if tx, err = db.db.Begin(); err != nil {
		db.log.Printf("[ERROR] Cannot begin transaction: %s\n",
			err.Error())
		return err
	}

	for _, q := range initQueries {
		db.log.Printf("[TRACE] Execute init query:\n%s\n",
			q)
		if _, err = tx.Exec(q); err != nil {
			db.log.Printf("[ERROR] Cannot execute init query: %s\n%s\n",
				err.Error(),
				q)
			if rbErr := tx.Rollback(); rbErr != nil {
				db.log.Printf("[CANTHAPPEN] Cannot rollback transaction: %s\n",
					rbErr.Error())
				return rbErr
			}
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		db.log.Printf("[CANTHAPPEN] Failed to commit init transaction: %s\n",
			err.Error())
		return err
	}

	return nil
} // func (db *Database) initialize() error

// Close closes the database.
// If there is a pending transaction, it is rolled back.
func (db *Database) Close() error {
	// I wonder if would make more snese to panic() if something goes wrong

	var err error

	if db.tx != nil {
		if err = db.tx.Rollback(); err != nil {
			db.log.Printf("[CRITICAL] Cannot roll back pending transaction: %s\n",
				err.Error())
			return err
		}
		db.tx = nil
	}

	for key, stmt := range db.queries {
		if err = stmt.Close(); err != nil {
			db.log.Printf("[CRITICAL] Cannot close statement handle %s: %s\n",
				key,
				err.Error())
			return err
		}
		delete(db.queries, key)
	}

	if err = db.db.Close(); err != nil {
		db.log.Printf("[CRITICAL] Cannot close database: %s\n",
			err.Error())
	}

	db.db = nil
	return nil
} // func (db *Database) Close() error

func (db *Database) getQuery(id query.ID) (*sql.Stmt, error) {
	var (
		stmt  *sql.Stmt
		found bool
		err   error
	)

	if stmt, found = db.queries[id]; found {
		return stmt, nil
	} else if _, found = dbQueries[id]; !found {
		return nil, fmt.Errorf("Unknown Query %d",
			id)
	}

	db.log.Printf("[TRACE] Prepare query %s\n", id)

PREPARE_QUERY:
	if stmt, err = db.db.Prepare(dbQueries[id]); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto PREPARE_QUERY
		}

		db.log.Printf("[ERROR] Cannor parse query %s: %s\n%s\n",
			id,
			err.Error(),
			dbQueries[id])
		return nil, err
	}

	db.queries[id] = stmt
	return stmt, nil
} // func (db *Database) getQuery(query.ID) (*sql.Stmt, error)

func (db *Database) resetSPNamespace() {
	db.spNameCounter = 1
	db.spNameCache = make(map[string]string)
} // func (db *Database) resetSPNamespace()

func (db *Database) generateSPName(name string) string {
	var spname = fmt.Sprintf("Savepoint%05d",
		db.spNameCounter)

	db.spNameCache[name] = spname
	db.spNameCounter++
	return spname
} // func (db *Database) generateSPName() string

// PerformMaintenance performs some maintenance operations on the database.
// It cannot be called while a transaction is in progress and will block
// pretty much all access to the database while it is running.
func (db *Database) PerformMaintenance() error {
	var mQueries = []string{
		"PRAGMA wal_checkpoint(TRUNCATE)",
		"VACUUM",
		"REINDEX",
		"ANALYZE",
	}
	var err error

	if db.tx != nil {
		return ErrTxInProgress
	}

	for _, q := range mQueries {
		if _, err = db.db.Exec(q); err != nil {
			db.log.Printf("[ERROR] Failed to execute %s: %s\n",
				q,
				err.Error())
		}
	}

	return nil
} // func (db *Database) PerformMaintenance() error

// Begin begins an explicit database transaction.
// Only one transaction can be in progress at once, attempting to start one,
// while another transaction is already in progress will yield ErrTxInProgress.
func (db *Database) Begin() error {
	var err error

	db.log.Printf("[DEBUG] Database#%d Begin Transaction\n",
		db.id)

	if db.tx != nil {
		return ErrTxInProgress
	}

BEGIN_TX:
	for db.tx == nil {
		if db.tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				continue BEGIN_TX
			} else {
				db.log.Printf("[ERROR] Failed to start transaction: %s\n",
					err.Error())
				return err
			}
		}
	}

	db.resetSPNamespace()

	return nil
} // func (db *Database) Begin() error

// SavepointCreate creates a savepoint with the given name.
//
// Savepoints only make sense within a running transaction, and just like
// with explicit transactions, managing them is the responsibility of the
// user of the Database.
//
// Creating a savepoint without a surrounding transaction is not allowed,
// even though SQLite allows it.
//
// For details on how Savepoints work, check the excellent SQLite
// documentation, but here's a quick guide:
//
// Savepoints are kind-of-like transactions within a transaction: One
// can create a savepoint, make some changes to the database, and roll
// back to that savepoint, discarding all changes made between
// creating the savepoint and rolling back to it. Savepoints can be
// quite useful, but there are a few things to keep in mind:
//
// - Savepoints exist within a transaction. When the surrounding transaction
//   is finished, all savepoints created within that transaction cease to exist,
//   no matter if the transaction is commited or rolled back.
//
// - When the database is recovered after being interrupted during a
//   transaction, e.g. by a power outage, the entire transaction is rolled back,
//   including all savepoints that might exist.
//
// - When a savepoint is released, nothing changes in the state of the
//   surrounding transaction. That means rolling back the surrounding
//   transaction rolls back the entire transaction, regardless of any
//   savepoints within.
//
// - Savepoints do not nest. Releasing a savepoint releases it and *all*
//   existing savepoints that have been created before it. Rolling back to a
//   savepoint removes that savepoint and all savepoints created after it.
func (db *Database) SavepointCreate(name string) error {
	var err error

	db.log.Printf("[DEBUG] SavepointCreate(%s)\n",
		name)

	if db.tx == nil {
		return ErrNoTxInProgress
	}

SAVEPOINT:
	// It appears that the SAVEPOINT statement does not support placeholders.
	// But I do want to used named savepoints.
	// And I do want to use the given name so that no SQL injection
	// becomes possible.
	// It would be nice if the database package or at least the SQLite
	// driver offered a way to escape the string properly.
	// One possible solution would be to use names generated by the
	// Database instead of user-defined names.
	//
	// But then I need a way to use the Database-generated name
	// in rolling back and releasing the savepoint.
	// I *could* use the names strictly inside the Database, store them in
	// a map or something and hand out a key to that name to the user.
	// Since savepoint only exist within one transaction, I could even
	// re-use names from one transaction to the next.
	//
	// Ha! I could accept arbitrary names from the user, generate a
	// clean name, and store these in a map. That way the user can
	// still choose names that are outwardly visible, but they do
	// not touch the Database itself.
	//
	//if _, err = db.tx.Exec("SAVEPOINT ?", name); err != nil {
	// if _, err = db.tx.Exec("SAVEPOINT " + name); err != nil {
	// 	if worthARetry(err) {
	// 		waitForRetry()
	// 		goto SAVEPOINT
	// 	}

	// 	db.log.Printf("[ERROR] Failed to create savepoint %s: %s\n",
	// 		name,
	// 		err.Error())
	// }

	var internalName = db.generateSPName(name)

	var spQuery = "SAVEPOINT " + internalName

	if _, err = db.tx.Exec(spQuery); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto SAVEPOINT
		}

		db.log.Printf("[ERROR] Failed to create savepoint %s: %s\n",
			name,
			err.Error())
	}

	return err
} // func (db *Database) SavepointCreate(name string) error

// SavepointRelease releases the Savepoint with the given name, and all
// Savepoints created before the one being release.
func (db *Database) SavepointRelease(name string) error {
	var (
		err                   error
		internalName, spQuery string
		validName             bool
	)

	db.log.Printf("[DEBUG] SavepointRelease(%s)\n",
		name)

	if db.tx != nil {
		return ErrNoTxInProgress
	}

	if internalName, validName = db.spNameCache[name]; !validName {
		db.log.Printf("[ERROR] Attempt to release unknown Savepoint %q\n",
			name)
		return ErrInvalidSavepoint
	}

	db.log.Printf("[DEBUG] Release Savepoint %q (%q)",
		name,
		db.spNameCache[name])

	spQuery = "RELEASE SAVEPOINT " + internalName

SAVEPOINT:
	if _, err = db.tx.Exec(spQuery); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto SAVEPOINT
		}

		db.log.Printf("[ERROR] Failed to release savepoint %s: %s\n",
			name,
			err.Error())
	} else {
		delete(db.spNameCache, internalName)
	}

	return err
} // func (db *Database) SavepointRelease(name string) error

// SavepointRollback rolls back the running transaction to the given savepoint.
func (db *Database) SavepointRollback(name string) error {
	var (
		err                   error
		internalName, spQuery string
		validName             bool
	)

	db.log.Printf("[DEBUG] SavepointRollback(%s)\n",
		name)

	if db.tx != nil {
		return ErrNoTxInProgress
	}

	if internalName, validName = db.spNameCache[name]; !validName {
		return ErrInvalidSavepoint
	}

	spQuery = "ROLLBACK TO SAVEPOINT " + internalName

SAVEPOINT:
	if _, err = db.tx.Exec(spQuery); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto SAVEPOINT
		}

		db.log.Printf("[ERROR] Failed to create savepoint %s: %s\n",
			name,
			err.Error())
	}

	delete(db.spNameCache, name)
	return err
} // func (db *Database) SavepointRollback(name string) error

// Rollback terminates a pending transaction, undoing any changes to the
// database made during that transaction.
// If no transaction is active, it returns ErrNoTxInProgress
func (db *Database) Rollback() error {
	var err error

	db.log.Printf("[DEBUG] Database#%d Roll back Transaction\n",
		db.id)

	if db.tx == nil {
		return ErrNoTxInProgress
	} else if err = db.tx.Rollback(); err != nil {
		return fmt.Errorf("Cannot roll back database transaction: %s",
			err.Error())
	}

	db.tx = nil
	db.resetSPNamespace()

	return nil
} // func (db *Database) Rollback() error

// Commit ends the active transaction, making any changes made during that
// transaction permanent and visible to other connections.
// If no transaction is active, it returns ErrNoTxInProgress
func (db *Database) Commit() error {
	var err error

	db.log.Printf("[DEBUG] Database#%d Commit Transaction\n",
		db.id)

	if db.tx == nil {
		return ErrNoTxInProgress
	} else if err = db.tx.Commit(); err != nil {
		return fmt.Errorf("Cannot commit transaction: %s",
			err.Error())
	}

	db.resetSPNamespace()
	db.tx = nil
	return nil
} // func (db *Database) Commit() error

// ProgramAdd adds a Program to the Database.
func (db *Database) ProgramAdd(p *objects.Program) error {
	const qid query.ID = query.ProgramAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var res sql.Result

EXEC_QUERY:
	if res, err = stmt.Exec(p.Title, p.Creator); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Program %s to database: %s",
				p.Title,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var programID int64

		if programID, err = res.LastInsertId(); err != nil {
			db.log.Printf("[ERROR] Cannot get ID of new Program %s: %s\n",
				p.Title,
				err.Error())
			return err
		}

		status = true
		p.ID = programID
		return nil
	}
} // func (db *Database) ProgramAdd(p *objects.Program) error

// ProgramDel removes a Program from the Database.
func (db *Database) ProgramDel(p *objects.Program) error {
	const qid query.ID = query.ProgramDel
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(p.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot delete Program %s (%d) from database: %s",
				p.Title,
				p.ID,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) ProgramDel(p *objects.Program) error

// ProgramSetTitle updates a Program's title.
func (db *Database) ProgramSetTitle(p *objects.Program, t string) error {
	const qid query.ID = query.ProgramSetTitle
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(t, p.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot set title for Program %q to %q: %s",
				p.Title,
				t,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	p.Title = t
	return nil
} // func (db *Database) ProgramSetTitle(p *objects.Program, t string) error

// ProgramSetCreator updates the Program's Creator.
func (db *Database) ProgramSetCreator(p *objects.Program, c string) error {
	const qid query.ID = query.ProgramSetCreator
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(c, p.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot set Creator for Program %q to %q: %s",
				p.Title,
				c,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	p.Creator = c
	return nil
} // func (db *Database) ProgramSetCreator(p *objects.Program, c string) error

// ProgramSetURL updates the Program's Creator.
func (db *Database) ProgramSetURL(p *objects.Program, u *url.URL) error {
	const qid query.ID = query.ProgramSetURL
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

	var s string

	if u == nil {
		s = ""
	} else {
		s = u.String()
	}

EXEC_QUERY:
	if _, err = stmt.Exec(s, p.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot update URL for Program %q to %q: %s",
				p.Title,
				s,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	p.URL = u
	return nil
} // func (db *Database) ProgramSetURL(p *objects.Program, u *url.URL) error

// ProgramSetCurFile updates the Program's index to the File being currently played.
func (db *Database) ProgramSetCurFile(p *objects.Program, f *objects.File) error {
	const qid query.ID = query.ProgramSetCurFile
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if f.ProgramID != p.ID {
		msg = fmt.Sprintf("File %q does not belong to Program %q",
			f.DisplayTitle(),
			p.Title)
		db.log.Printf("[ERROR] %s\n", msg)
		return errors.New(msg)
	} else if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(f.ID, p.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot update current File for Program %q to %q: %s",
				p.Title,
				f.DisplayTitle(),
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	p.CurFile = f.ID
	return nil
} // func (db *Database) ProgramSetCurFile(p *objects.Program, f *objects.File) error

// ProgramGetByID loads a Program by its Database ID.
func (db *Database) ProgramGetByID(id int64) (*objects.Program, error) {
	const qid query.ID = query.ProgramGetByID
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(id); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			u string
			p = &objects.Program{ID: id}
		)

		if err = rows.Scan(&p.Title, &p.Creator, &u, &p.CurFile); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		} else if p.URL, err = url.Parse(u); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				u,
				err.Error())
			return nil, err
		}

		return p, nil
	}

	return nil, nil
} // func (db *Database) ProgramGetByID(id int64) (*objects.Program, error)

// ProgramGetByTitle looks up a Program by its Title. Returns nil, if no
// such program is found. The title has to be an exact, case-sensitivity and all.
func (db *Database) ProgramGetByTitle(title string) (*objects.Program, error) {
	const qid query.ID = query.ProgramGetByTitle
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(title); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			u string
			p = &objects.Program{Title: title}
		)

		if err = rows.Scan(&p.ID, &p.Creator, &u, &p.CurFile); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		} else if p.URL, err = url.Parse(u); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				u,
				err.Error())
			return nil, err
		}

		return p, nil
	}

	return nil, nil
} // func (db *Database) ProgramGetByTitle(title string) (*objects.Program, error)

// ProgramGetAll loads all Programs from the Database.
func (db *Database) ProgramGetAll() ([]objects.Program, error) {
	const qid query.ID = query.ProgramGetAll
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var programs = make([]objects.Program, 0)

	for rows.Next() {
		var (
			u string
			p objects.Program
		)

		if err = rows.Scan(&p.ID, &p.Title, &p.Creator, &u, &p.CurFile); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		} else if p.URL, err = url.Parse(u); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				u,
				err.Error())
			return nil, err
		}

		programs = append(programs, p)
	}

	return programs, nil
} // func (db *Database) ProgramGetAll() ([]objects.Program, error)

// FileAdd adds a File to the Database.
func (db *Database) FileAdd(f *objects.File) error {
	const qid query.ID = query.FileAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var res sql.Result

EXEC_QUERY:
	if res, err = stmt.Exec(f.Path, f.FolderID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Program %s to database: %s",
				f.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var fileID int64

		if fileID, err = res.LastInsertId(); err != nil {
			db.log.Printf("[ERROR] Cannot get ID of new Program %s: %s\n",
				f.Path,
				err.Error())
			return err
		}

		status = true
		f.ID = fileID
		return nil
	}
} // func (db *Database) FileAdd(f *objects.File) error

// FileDel deletes a File from the Database.
func (db *Database) FileDel(f *objects.File) error {
	const qid query.ID = query.FileDel
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot delete Program %s (%d) from database: %s",
				f.Title,
				f.ID,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) FileDel(f *objects.File) error

// FileGetByID looks up a File by its ID.
func (db *Database) FileGetByID(id int64) (*objects.File, error) {
	const qid query.ID = query.FileGetByID
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(id); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			stamp int64
			f     = &objects.File{ID: id}
		)

		if err = rows.Scan(&f.ProgramID, &f.FolderID, &f.Path, &f.Title, &f.Position, &stamp); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		}

		f.LastPlayed = time.Unix(stamp, 0)

		return f, nil
	}

	return nil, nil
} // func (db *Database) FileGetByID(id int64) (*objects.File, error)

// FileGetByPath looks up a File by its file system path.
func (db *Database) FileGetByPath(path string) (*objects.File, error) {
	const qid query.ID = query.FileGetByPath
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(path); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			stamp int64
			f     = &objects.File{Path: path}
		)

		if err = rows.Scan(&f.ID, &f.ProgramID, &f.FolderID, &f.Title, &f.Position, &stamp); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		}

		f.LastPlayed = time.Unix(stamp, 0)

		return f, nil
	}

	return nil, nil
} // func (db *Database) FileGetByPath(path string) (*objects.File, error)

// FileGetByProgram looks up all the files belonging to the given Program.
func (db *Database) FileGetByProgram(p *objects.Program) ([]objects.File, error) {
	const qid query.ID = query.FileGetByProgram
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(p.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	var res = make([]objects.File, 0, 8)

	for rows.Next() {
		var (
			stamp int64
			f     = objects.File{ProgramID: p.ID}
		)

		if err = rows.Scan(
			&f.ID,
			&f.FolderID,
			&f.Path,
			&f.Title,
			&f.Position,
			&stamp); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		}

		f.LastPlayed = time.Unix(stamp, 0)

		res = append(res, f)
	}

	return res, nil
} // func (db *Database) FileGetByProgram(p *objects.Program) ([]objects.File, error)

// FileGetNoProgram looks up all the files that have not been assigned to a Program.
func (db *Database) FileGetNoProgram() ([]objects.File, error) {
	const qid query.ID = query.FileGetNoProgram
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	var res = make([]objects.File, 0, 8)

	for rows.Next() {
		var (
			stamp int64
			f     objects.File
		)

		if err = rows.Scan(
			&f.ID,
			&f.FolderID,
			&f.Path,
			&f.Title,
			&f.Position,
			&stamp); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		}

		f.LastPlayed = time.Unix(stamp, 0)

		res = append(res, f)
	}

	return res, nil
} // func (db *Database) FileGetNoProgram() ([]objects.File, error)

// FileSetTitle update the title of a file.
func (db *Database) FileSetTitle(f *objects.File, title string) error {
	const qid query.ID = query.FileSetTitle
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(title, f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot set Title of File %q (%d) to %q: %s",
				f.DisplayTitle(),
				f.ID,
				title,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	f.Title = title
	return nil
} // func (db *Database) FileSetTitle(f *objects.File, title string) error

// FileSetPosition sets the playback position of a File.
func (db *Database) FileSetPosition(f *objects.File, pos int64) error {
	const qid query.ID = query.FileSetPosition
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var now = time.Now()

EXEC_QUERY:
	if _, err = stmt.Exec(pos, now.Unix(), f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot set Position of File %q (%d) to %d: %s",
				f.DisplayTitle(),
				f.ID,
				pos,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	f.Position = pos
	f.LastPlayed = now
	return nil
} // func (db *Database) FileSetPosition(f *objects.File, pos int64) error

// FileSetProgram sets a File's ProgramID to the one of the given Program.
func (db *Database) FileSetProgram(f *objects.File, p *objects.Program) error {
	const qid query.ID = query.FileSetProgram
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

	var pid *int64

	if p != nil {
		pid = &p.ID
	}

EXEC_QUERY:
	if _, err = stmt.Exec(pid, f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			var pTitle string
			if p != nil {
				pTitle = p.Title
			} else {
				pTitle = "(NULL)"
			}
			err = fmt.Errorf("Cannot set Program of File %q (%d) to %s: %s",
				f.DisplayTitle(),
				f.ID,
				pTitle,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	if p != nil {
		f.ProgramID = p.ID
	} else {
		f.ProgramID = 0
	}
	return nil
} // func (db *Database) FileSetProgram(f *objects.File, p *objects.Program) error

// FolderAdd adds a new Folder to the database.
func (db *Database) FolderAdd(f *objects.Folder) error {
	const qid query.ID = query.FolderAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var res sql.Result

EXEC_QUERY:
	if res, err = stmt.Exec(f.Path); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Program %s to database: %s",
				f.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var folderID int64

		if folderID, err = res.LastInsertId(); err != nil {
			db.log.Printf("[ERROR] Cannot get ID of new Program %s: %s\n",
				f.Path,
				err.Error())
			return err
		}

		status = true
		f.ID = folderID
		return nil
	}
} // func (db *Database) FolderAdd(f *objects.Folder) error

// FolderUpdateScan updates a Folder's last_scan timestamp.
func (db *Database) FolderUpdateScan(f *objects.Folder, t time.Time) error {
	const qid query.ID = query.FolderUpdateScan
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid.String(),
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(t.Unix(), f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Program %s to database: %s",
				f.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		status = true
		f.LastScan = t
		return nil
	}
} // func (db *Database) FolderUpdateScan(f *objects.Folder, t time.Time) error

// FolderGetByPath looks up a Folder by its Path.
func (db *Database) FolderGetByPath(path string) (*objects.Folder, error) {
	const qid query.ID = query.FolderGetByPath
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(path); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			stamp int64
			f     = &objects.Folder{Path: path}
		)

		if err = rows.Scan(&f.ID, &stamp); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		}

		f.LastScan = time.Unix(stamp, 0)

		return f, nil
	}

	return nil, nil
} // func (db *Database) FolderGetByPath(path string) (*objects.Folder, error)

// FolderGetByID looks up a Folder by its ID
func (db *Database) FolderGetByID(id int64) (*objects.Folder, error) {
	const qid query.ID = query.FolderGetByID
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(id); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			stamp int64
			f     = &objects.Folder{ID: id}
		)

		if err = rows.Scan(&f.Path, &stamp); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		}

		f.LastScan = time.Unix(stamp, 0)

		return f, nil
	}

	return nil, nil
} // func (db *Database) FolderGetByID(id int64) (*objects.Folder, error)

// FolderGetAll return all Folders from the database.
func (db *Database) FolderGetAll() ([]objects.Folder, error) {
	const qid query.ID = query.FolderGetAll
	var (
		err  error
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	var folders = make([]objects.Folder, 0, 4)

	for rows.Next() {
		var (
			stamp int64
			f     objects.Folder
		)

		if err = rows.Scan(&f.ID, &f.Path, &stamp); err != nil {
			db.log.Printf("[ERROR] Cannot scan row: %s\n", err.Error())
			return nil, err
		}

		f.LastScan = time.Unix(stamp, 0)
		folders = append(folders, f)
	}

	return folders, nil
} // func (db *Database) FolderGetAll() ([]objects.Folder, error)

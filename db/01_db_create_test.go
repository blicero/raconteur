// /home/krylon/go/src/github.com/blicero/raconteur/db/01_db_create_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 31. 05. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-05-31 20:28:48 krylon>

package db

import (
	"database/sql"
	"testing"

	"github.com/blicero/raconteur/common"
)

func TestCreateDB(t *testing.T) {
	var err error

	if conn, err = Open(common.DbPath); err != nil {
		conn = nil
		t.Fatalf("Cannot create database at %s: %s",
			common.DbPath,
			err.Error())
	}
} // func TestCreateDB(t *testing.T)

func TestPrepareQueries(t *testing.T) {
	if conn == nil {
		t.SkipNow()
	}

	for qid := range dbQueries {
		var (
			e error
			s *sql.Stmt
		)

		if s, e = conn.getQuery(qid); e != nil {
			t.Errorf("Error preparing query %s: %s",
				qid,
				e.Error())
		} else if s == nil {
			t.Errorf("getQuery(%s) returned no error but nil",
				qid)
		}
	}
} // func TestPrepareQueries(t *testing.T)

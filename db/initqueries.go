// /home/krylon/go/src/github.com/blicero/raconteur/db/initqueries.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2021-09-07 19:02:15 krylon>

package db

var initQueries = []string{
	`
CREATE TABLE program (
    id                   INTEGER PRIMARY KEY,
    title                TEXT UNIQUE NOT NULL,
    creator              TEXT,
)
`,
	`
CREATE TABLE file (
    id                   INTEGER PRIMARY KEY,
    program_id           INTEGER,
    path                 TEXT UNIQUE NOT NULL,
    title                TEXT,
    position             TEXT,
    last_played          INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (program_id) REFERENCES program (id)
)
`,
	"CREATE INDEX file_prog_idx ON file (program_id)",
}

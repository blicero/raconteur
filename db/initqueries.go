// /home/krylon/go/src/github.com/blicero/raconteur/db/initqueries.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-20 17:52:10 krylon>

package db

var initQueries = []string{
	`
CREATE TABLE folder (
    id			INTEGER PRIMARY KEY,
    path		TEXT UNIQUE NOT NULL,
    last_scan           INTEGER NOT NULL DEFAULT 0,
    CHECK (path LIKE '/%')
)
`,
	`
CREATE TABLE program (
    id                   INTEGER PRIMARY KEY,
    title                TEXT UNIQUE NOT NULL,
    creator              TEXT NOT NULL DEFAULT '',
    url                  TEXT NOT NULL DEFAULT ''
)
`,
	`
CREATE TABLE file (
    id                   INTEGER PRIMARY KEY,
    program_id           INTEGER,
    folder_id            INTEGER NOT NULL,
    path                 TEXT UNIQUE NOT NULL,
    title                TEXT NOT NULL DEFAULT '',
    position             INTEGER NOT NULL DEFAULT 0,
    last_played          INTEGER NOT NULL DEFAULT 0,
    url                  TEXT,
    FOREIGN KEY (program_id) REFERENCES program (id)
        ON DELETE CASCADE
        ON UPDATE RESTRICT,
    FOREIGN KEY (folder_id) REFERENCES folder (id)
        ON DELETE CASCADE
        ON UPDATE RESTRICT
)
`,
	"CREATE INDEX file_prog_idx ON file (program_id)",
	"CREATE INDEX file_path_idx ON file (path)",
	"CREATE INDEX file_title_idx ON file (title)",
}

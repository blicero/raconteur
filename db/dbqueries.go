// /home/krylon/go/src/github.com/blicero/raconteur/db/dbqueries.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-02 19:20:35 krylon>

package db

import "github.com/blicero/raconteur/db/query"

var dbQueries = map[query.ID]string{
	query.ProgramAdd:        "INSERT INTO program (title, creator) VALUES (?, ?)",
	query.ProgramDel:        "DELETE FROM program WHERE id = ?",
	query.ProgramGetAll:     "SELECT id, title, creator FROM program",
	query.ProgramGetByID:    "SELECT title, creator FROM program WHERE id = ?",
	query.ProgramGetByTitle: "SELECT id, creator FROM program WHERE title = ?",
	query.FileAdd:           "INSERT INTO file (path) VALUES (?)",
	query.FileDel:           "DELETE FROM file WHERE id = ?",
	query.FileGetByID:       "SELECT program_id, path, title, position, last_played FROM file WHERE id = ?",
	query.FileGetByPath:     "SELECT id, program_id, title, position, last_played FROM file WHERE path = ?",
	query.FileGetByProgram:  "SELECT id, path, title, position, last_played FROM file WHERE program_id = ?",
	query.FileGetNoProgram:  "SELECT id, path, title, position, last_played FROM file WHERE program_id IS NULL",
	query.FileSetTitle:      "UPDATE file SET title = ? WHERE id = ?",
	query.FileSetPosition:   "UPDATE file SET position = ? WHERE id = ?",
	query.FileSetProgram:    "UPDATE file SET program_id = ? WHERE id = ?",
}

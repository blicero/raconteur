// /home/krylon/go/src/github.com/blicero/raconteur/db/query/query.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-07 19:38:19 krylon>

// Package query provides symbolic constants to identify SQL queries.
package query

//go:generate stringer -type=ID

type ID uint8

const (
	ProgramAdd ID = iota
	ProgramDel
	ProgramGetAll
	ProgramGetByID
	ProgramGetByTitle
	ProgramSetTitle
	ProgramSetURL
	ProgramSetCreator
	FileAdd
	FileDel
	FileGetByID
	FileGetByPath
	FileGetByProgram
	FileGetNoProgram
	FileSetTitle
	FileSetPosition
	FileSetProgram
)

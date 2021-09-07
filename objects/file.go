// /home/krylon/go/src/github.com/blicero/raconteur/objects/file.go
// -*- mode: go; coding: utf-8; -*-
// Created on 06. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2021-09-06 22:06:57 krylon>

package objects

import "time"

// File represents an audio file
type File struct {
	ID         int64
	ProgramID  int64
	Path       string
	Title      string
	Order      []int
	Position   int
	LastPlayed time.Time
}

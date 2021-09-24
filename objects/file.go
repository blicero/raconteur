// /home/krylon/go/src/github.com/blicero/raconteur/objects/file.go
// -*- mode: go; coding: utf-8; -*-
// Created on 06. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2021-09-11 20:12:38 krylon>

package objects

import (
	"path"
	"time"
)

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

// DisplayTitle returns a - somewhat - presentable string to represent the file.
func (f *File) DisplayTitle() string {
	if f.Title != "" {
		return f.Title
	} else {
		return path.Base(f.Path)
	}
} // func (f *File) DisplayTitle() string

// /home/krylon/go/src/github.com/blicero/raconteur/objects/folder.go
// -*- mode: go; coding: utf-8; -*-
// Created on 20. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-20 19:45:26 krylon>

package objects

import "time"

type Folder struct {
	ID       int64
	Path     string
	LastScan time.Time
}

// Clone return a pointer to a freshly-allocated memberwise copy of the receiver.
func (f *Folder) Clone() *Folder {
	return &Folder{
		ID:       f.ID,
		Path:     f.Path,
		LastScan: f.LastScan,
	}
} // func (f *Folder) Clone() *Folder

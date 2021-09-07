// /home/krylon/go/src/github.com/blicero/raconteur/objects/program.go
// -*- mode: go; coding: utf-8; -*-
// Created on 06. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2021-09-06 22:16:47 krylon>

package objects

// Program is a rather generic term for any sequence of one or more audio
// files that one might enjoy listening to, be it an audio book, a podcast, or
// whatever.
type Program struct {
	ID      int64
	Title   string
	Creator string
}

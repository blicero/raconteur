// /home/krylon/go/src/github.com/blicero/raconteur/objects/program.go
// -*- mode: go; coding: utf-8; -*-
// Created on 06. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-07 19:27:09 krylon>

package objects

import "net/url"

// Program is a rather generic term for any sequence of one or more audio
// files that one might enjoy listening to, be it an audio book, a podcast, or
// whatever.
type Program struct {
	ID      int64
	Title   string
	Creator string
	URL     *url.URL
}

// URLString returns the Program's associated URL as a string.
func (p *Program) URLString() string {
	if p.URL != nil {
		return p.URL.String()
	}

	return ""
} // func (p *Program) URLString() string

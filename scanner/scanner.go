// /home/krylon/go/src/github.com/blicero/raconteur/scanner/scanner.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2021-09-07 21:14:26 krylon>

// Package scanner implements processing directory trees looking for files that,
// allegedly, are podcast episodes, audio books, or parts of audio books.
package scanner

import (
	"log"
	"regexp"
)

var suffixPattern = regexp.MustCompile("[.](?:mp3|m4[ab]|mpga|og[agm]|opus|wma|flac)$")

type walker struct {
	log  *log.Logger
	root string
}

// /home/krylon/go/src/github.com/blicero/raconteur/scanner/scanner.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-05-31 20:21:38 krylon>

// Package scanner implements processing directory trees looking for files that,
// allegedly, are podcast episodes, audio books, or parts of audio books.
package scanner

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"regexp"

	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/logdomain"
	"github.com/blicero/raconteur/objects"
)

const (
	minSize = 8 * 1024 * 1024 // 8 MiB
)

var suffixPattern = regexp.MustCompile("[.](?:mp3|m4[ab]|mpga|og[agm]|opus|wma|flac)$")

// Walker implements traversing directory trees to find audio files.
type Walker struct {
	log   *log.Logger
	root  string
	db    *db.Database
	Files chan *objects.File
}

// New creates a new Walker for the given folder.
func New(root string, conn *db.Database) (*Walker, error) {
	var w = &Walker{
		root:  root,
		db:    conn,
		Files: make(chan *objects.File, 8),
	}
	var err error

	if w.log, err = common.GetLogger(logdomain.Scanner); err != nil {
		fmt.Fprintf(os.Stderr,
			"Error getting Logger for %s: %s\n",
			logdomain.Scanner,
			err.Error())
		return nil, err
	}

	return w, nil
} // func New(root string) (*Walker, error)

// Walk initiates the traversal of the Walker's directory tree.
func (w *Walker) Walk() {

} // func (w *Walker) Walk()

func (w *Walker) visit(path string, d fs.DirEntry, incoming error) error {
	if incoming != nil {
		w.log.Printf("[INFO] Incoming error for %s: %s\n",
			path,
			incoming.Error())
		if d.IsDir() {
			return nil // fs.SkipDir
		} else {
			return nil
		}
	}

	var (
		info fs.FileInfo
		err  error
		file *objects.File
	)

	if !suffixPattern.MatchString(path) {
		return nil
	} else if info, err = d.Info(); err != nil {
		return err
	} else if info.Size() < minSize {
		return nil
	} else if file, err = w.db.FileGetByPath(path); err != nil {
		w.log.Printf("[ERROR] Failed to look up file %s: %s\n",
			path,
			err.Error())
		return nil
	} else if file == nil {
		file = &objects.File{
			Path: path,
		}
	}

	return nil
} // func (w *Walker) visit(path string, d fs.DirEntry, err error) error

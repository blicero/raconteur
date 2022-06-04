// /home/krylon/go/src/github.com/blicero/raconteur/scanner/scanner.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-04 16:43:53 krylon>

// Package scanner implements processing directory trees looking for files that,
// allegedly, are podcast episodes, audio books, or parts of audio books.
package scanner

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/blicero/krylib"
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
	lock  sync.Mutex
	db    *db.Database
	root  string
	Files chan *objects.File
}

// New creates a new Walker for the given folder.
func New(conn *db.Database) (*Walker, error) {
	var w = &Walker{
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
func (w *Walker) Walk(root string) error {
	w.log.Printf("[INFO] Scan %s\n", root)
	defer w.log.Printf("[INFO] Done scanning %s\n", root)

	w.lock.Lock()
	defer w.lock.Unlock()

	w.root = root
	defer func() { w.root = "" }()

	return fs.WalkDir(os.DirFS(root), ".", w.visit)
} // func (w *Walker) Walk(root string) error

func (w *Walker) visit(path string, d fs.DirEntry, incoming error) error {
	if incoming != nil {
		w.log.Printf("[INFO] Incoming error for %s: %s\n",
			path,
			incoming.Error())
		return nil // incoming
	}

	if d.IsDir() {
		return nil
	}

	var (
		info     fs.FileInfo
		err      error
		f        *objects.File
		fullPath = filepath.Join(w.root, path)
	)

	w.log.Printf("[DEBUG] Process %s\n", fullPath)

	if !suffixPattern.MatchString(path) {
		return nil
	} else if info, err = d.Info(); err != nil {
		return err
	} else if info.Size() < minSize {
		w.log.Printf("[DEBUG] %s is too small (%s)\n",
			fullPath,
			krylib.FmtBytes(info.Size()))
		return nil
	} else if f, err = w.db.FileGetByPath(fullPath); err != nil {
		w.log.Printf("[ERROR] Failed to look up file %s: %s\n",
			fullPath,
			err.Error())
		return nil
	} else if f == nil {
		f = &objects.File{
			Path: fullPath,
		}

		if err = w.db.FileAdd(f); err != nil {
			w.log.Printf("[ERROR] Cannot add file %s to database: %s\n",
				path,
				err.Error())
			return err
		}
	}

	w.Files <- f

	return nil
} // func (w *Walker) visit(path string, d fs.DirEntry, err error) error

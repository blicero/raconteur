// /home/krylon/go/src/github.com/blicero/raconteur/scanner/scanner.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2023-09-08 22:00:25 krylon>

// Package scanner implements processing directory trees looking for files that,
// allegedly, are podcast episodes, audio books, or parts of audio books.
package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/logdomain"
	"github.com/blicero/raconteur/objects"
)

// minSize is the minimum file size required to process files.
// Files smaller than minSize are ignored.
const (
	minSize = 100 * 1024 // 100 KiB
)

var suffixPattern = regexp.MustCompile("[.](?:mp3|m4[ab]|mpga|og[agm]|opus|wma|flac)$")

// Walker implements traversing directory trees to find audio files.
type Walker struct {
	log       *log.Logger
	lock      sync.Mutex
	db        *db.Database
	root      string
	folder    *objects.Folder
	skipCache map[string]bool
	Files     chan *objects.File
}

// New creates a new Walker for the given folder.
func New(conn *db.Database) (*Walker, error) {
	var (
		err error
		w   = &Walker{
			db:        conn,
			Files:     make(chan *objects.File, 8),
			skipCache: make(map[string]bool),
		}
	)

	if conn == nil {
		fmt.Fprintf(
			os.Stderr,
			"Database connection is nil\n",
		)
		return nil, errors.New("Database connection is nil")
	} else if w.log, err = common.GetLogger(logdomain.Scanner); err != nil {
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
	w.lock.Lock()
	defer w.lock.Unlock()

	w.log.Printf("[INFO] Scan %s\n", root)
	defer w.log.Printf("[INFO] Done scanning %s\n", root)

	w.root = root
	defer func() { w.root = "" }()

	var (
		err    error
		folder *objects.Folder
	)

	if folder, err = w.db.FolderGetByPath(root); err != nil {
		w.log.Printf("[ERROR] Cannot look up Folder %s: %s\n",
			root,
			err.Error())
		return err
	} else if folder != nil {
		w.folder = folder
	} else {
		folder = &objects.Folder{Path: root}
		if err = w.db.FolderAdd(folder); err != nil {
			w.log.Printf("[ERROR] Failed to add Folder %s to database: %s\n",
				root,
				err.Error())
			return err
		}

		w.folder = folder
	}

	defer func() { w.folder = nil }()

	if err = fs.WalkDir(os.DirFS(root), ".", w.visit); err != nil {
		w.log.Printf("[ERROR] Error processing folder %q: %s\n",
			root,
			err.Error())
		return err
	} else if err = w.db.FolderUpdateScan(folder, time.Now()); err != nil {
		w.log.Printf("[ERROR] Cannot update scan timestamp on folder %q: %s\n",
			root,
			err.Error())
		return err
	}

	return nil
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

	if w.skipCache[path] {
		return nil
	} else if !suffixPattern.MatchString(path) {
		w.skipCache[path] = true
		return nil
	} else if info, err = d.Info(); err != nil {
		return err
	} else if info.Size() < minSize {
		w.skipCache[path] = true
		// w.log.Printf("[DEBUG] %s is too small (%s)\n",
		// 	fullPath,
		// 	krylib.FmtBytes(info.Size()))
		return nil
	} else if f, err = w.db.FileGetByPath(fullPath); err != nil {
		w.log.Printf("[ERROR] Failed to look up file %s: %s\n",
			fullPath,
			err.Error())
		return nil
	} else if f != nil {
		w.skipCache[path] = true
		return nil
	} else if f == nil {
		f = &objects.File{
			Path:     fullPath,
			FolderID: w.folder.ID,
			Ord:      make([]int64, 2),
		}

		if err = w.db.FileAdd(f); err != nil {
			w.log.Printf("[ERROR] Cannot add file %s to database: %s\n",
				path,
				err.Error())
			return err
		}

		w.log.Printf("[DEBUG] Process %s\n", fullPath)
		w.skipCache[path] = true
	}

	var (
		txStatus bool
		album    string
		meta     *metadata
	)

	if meta, err = getMetaAudio(f); err != nil {
		w.log.Printf("[ERROR] Cannot extract metadata from %s: %s\n",
			f.Path,
			err.Error())
		goto SEND_FILE
	} else if meta == nil {
		w.log.Printf("[INFO] No metadata found for %s\n",
			f.Path)
		goto SEND_FILE
	} else if err = w.db.Begin(); err != nil {
		w.log.Printf("[ERROR] Cannot start transaction: %s\n",
			err.Error())
		goto SEND_FILE
	}

	if title := meta.Title; title != "" {
		if err = w.db.FileSetTitle(f, title); err != nil {
			w.log.Printf("[ERROR] Cannot set Title for File %s to %q: %s\n",
				f.Path,
				title,
				err.Error())
			goto FINISH_TX
		}
	}

	if album = meta.Album; album == "" {
		album = f.GetParentFolder()
		if album == filepath.Base(w.root) {
			album = ""
		}
	}

	if album != "" {
		var prog *objects.Program

		if prog, err = w.db.ProgramGetByTitle(album); err != nil {
			w.log.Printf("[ERROR] Failed to look for Program %q: %s\n",
				album,
				err.Error())
			goto FINISH_TX
		} else if prog == nil {
			prog = &objects.Program{
				Title:   album,
				Creator: meta.Artist,
			}

			if err = w.db.ProgramAdd(prog); err != nil {
				w.log.Printf("[ERROR] Cannot add Program %s: %s\n",
					album,
					err.Error())
				goto FINISH_TX
			}
		}

		if err = w.db.FileSetProgram(f, prog); err != nil {
			w.log.Printf("[ERROR] Cannot set Program for File %q to %q: %s\n",
				f.Path,
				album,
				err.Error())
			goto FINISH_TX
		}
	}

	if meta.Ord[0] != 0 || meta.Ord[1] != 0 {
		if err = w.db.FileSetOrd(f, meta.Ord); err != nil {
			w.log.Printf("[ERROR] Cannot set track number for File %q: %s\n",
				f.DisplayTitle(),
				err.Error())
			goto FINISH_TX
		}
	}

	txStatus = true

FINISH_TX:
	if txStatus {
		w.db.Commit() // nolint: errcheck
	} else {
		w.db.Rollback() // nolint: errcheck
	}

SEND_FILE:
	w.Files <- f

	return nil
} // func (w *Walker) visit(path string, d fs.DirEntry, err error) error

// /home/krylon/go/src/github.com/blicero/raconteur/objects/file.go
// -*- mode: go; coding: utf-8; -*-
// Created on 06. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-23 19:30:20 krylon>

package objects

import (
	"net/url"
	"path"
	"path/filepath"
	"time"
)

// File represents an audio file
type File struct {
	ID         int64
	ProgramID  int64
	FolderID   int64
	Path       string
	Title      string
	URL        string
	Order      []int
	Position   int64
	LastPlayed time.Time
}

// DisplayTitle returns a - somewhat - presentable string to represent the file.
func (f *File) DisplayTitle() string {
	if f.Title != "" {
		return f.Title
	}

	return path.Base(f.Path)
} // func (f *File) DisplayTitle() string

// Duration returns the duration of a File.
func (f *File) Duration() (time.Duration, error) {
	return 0, nil
} // func (f *File) Duration() (time.Duration, error)

// Clone returns an identical copy of the receiver.
func (f *File) Clone() *File {
	var ord = make([]int, len(f.Order))

	for idx, o := range f.Order {
		ord[idx] = o
	}

	return &File{
		ID:         f.ID,
		ProgramID:  f.ProgramID,
		Path:       f.Path,
		Title:      f.Title,
		URL:        f.URL,
		Order:      ord,
		Position:   f.Position,
		LastPlayed: f.LastPlayed,
	}
} // func (f *File) Clone() *File

// GetParentFolder returns the name of the Folder the file lives in,
// i.e. basename(dirname(path))
func (f *File) GetParentFolder() string {
	return filepath.Base(filepath.Dir(f.Path))
} // func (f *File) GetParentFolder() string

// PathURL returns the File's path as a file:// URL.
// Intended for use with DBus interfaces
func (f *File) PathURL() string {
	return "file://" + url.PathEscape(f.Path)
} // func (f *File) PathURL() string

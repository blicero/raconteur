// /home/krylon/go/src/github.com/blicero/raconteur/scanner/meta.go
// -*- mode: go; coding: utf-8; -*-
// Created on 14. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-14 18:15:09 krylon>

package scanner

import (
	"os"
	"strconv"

	"github.com/dhowden/tag"
)

// This file contains functions to extract metadata from Files.

// getMetaAudio extracts metadata from various audio formats.
func getMetaAudio(f *File) (map[string]string, error) {
	var (
		fh   *os.File
		meta map[string]string
		m    tag.Metadata
		err  error
	)

	if fh, err = os.Open(f.Path); err != nil {
		return nil, err
	}

	defer fh.Close() // nolint: errcheck

	if m, err = tag.ReadFrom(fh); err != nil {
		return nil, err
	}

	meta = map[string]string{
		"Title":  m.Title(),
		"Artist": m.Artist(),
		"Album":  m.Album(),
		"Year":   strconv.Itoa(m.Year()),
	}

	return meta, nil
} // func GetMetaAudio(f *File) (map[string]string, error)

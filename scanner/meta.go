// /home/krylon/go/src/github.com/blicero/raconteur/scanner/meta.go
// -*- mode: go; coding: utf-8; -*-
// Created on 14. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-25 20:50:55 krylon>

package scanner

import (
	"os"

	"github.com/blicero/raconteur/objects"
	"github.com/dhowden/tag"
)

// This file contains functions to extract metadata from Files.

type metadata struct {
	Title  string
	Artist string
	Album  string
	Year   int64
	Ord    []int64
}

// getMetaAudio extracts metadata from various audio formats.
func getMetaAudio(f *objects.File) (*metadata, error) {
	var (
		fh   *os.File
		meta *metadata
		m    tag.Metadata
		ord  [2]int
		err  error
	)

	if fh, err = os.Open(f.Path); err != nil {
		return nil, err
	}

	defer fh.Close() // nolint: errcheck

	if m, err = tag.ReadFrom(fh); err != nil {
		return nil, err
	}

	ord[1], _ = m.Track()
	ord[0], _ = m.Disc()

	meta = &metadata{
		Title:  m.Title(),
		Artist: m.Artist(),
		Album:  m.Album(),
		Year:   int64(m.Year()),
		Ord:    []int64{int64(ord[0]), int64(ord[1])},
	}

	return meta, nil
} // func GetMetaAudio(f *File) (map[string]string, error)

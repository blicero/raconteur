// /home/krylon/go/src/github.com/blicero/raconteur/scanner/meta.go
// -*- mode: go; coding: utf-8; -*-
// Created on 14. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2023-09-12 20:02:08 krylon>

package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

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

var episodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(?:\d+-)?(\d+)`),
	regexp.MustCompile("(?i)(?:ep(?:isode)?|show)?\\s*(\\d+)"),
	regexp.MustCompile("(?i)^[A-Z]{1,4}\\s?(\\d+)"),
	regexp.MustCompile("(?i)ep[.]?\\s+(\\d+)"),
	regexp.MustCompile("^\\s*#(\\d+)"),
	regexp.MustCompile("Folge (\\d+)"),
}

func extractEpisodeNumber(path string) (int, bool) {
	var (
		err error
		n   int64
		m   []string
	)

	for _, p := range episodePatterns {
		if m = p.FindStringSubmatch(path); len(m) > 0 {
			if n, err = strconv.ParseInt(m[1], 10, 64); err != nil {
				fmt.Fprintf(
					os.Stderr,
					"Cannot parse integer %q: %s\n",
					m[1],
					err.Error())
				continue
			}

			return int(n), true
		}
	}

	return 0, false
} // func extractEpisodeNumber(path string) (int, bool)

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

	if ord[1] == 0 {
		// We can still try to extract the number from the file name.
		ord[1], _ = extractEpisodeNumber(filepath.Base(f.Path))
	}

	meta = &metadata{
		Title:  m.Title(),
		Artist: m.Artist(),
		Album:  m.Album(),
		Year:   int64(m.Year()),
		Ord:    []int64{int64(ord[0]), int64(ord[1])},
	}

	return meta, nil
} // func GetMetaAudio(f *File) (map[string]string, error)

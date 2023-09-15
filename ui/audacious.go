// /home/krylon/go/src/github.com/blicero/raconteur/ui/audacious.go
// -*- mode: go; coding: utf-8; -*-
// Created on 08. 09. 2023 by Benjamin Walkenhorst
// (c) 2023 Benjamin Walkenhorst
// Time-stamp: <2023-09-15 14:10:44 krylon>

package ui

// We try to use audacious as our player. It offers a much richer DBus API than
// MPris2 does, so I'll try to use that to its fullest.

var songFields = []string{ // nolint: unused
	"title",
	"artist",
	"album",
	"album-artist",
	"comment",
	"genre",
	"year",
	"composer",
	"performer",
	"copyright",
	"date",
	"track-number",
	"length",
	"bitrate",
	"codec",
	"quality",
	"file-name",
	"file-path",
	"file-ext",
	"audio-file",
	"subsong-id",
	"subsong-num",
	"segment-start",
	"segment-end",
	"gain-album-gain",
	"gain-album-peak",
	"gain-track-gain",
	"gain-track-peak",
	"gain-gain-unit",
	"gain-peak-unit",
	"formatted-title",
	"description",
	"musicbrainz-id",
	"channels",
	"publisher",
	"catalog-number",
	"lyrics",
}

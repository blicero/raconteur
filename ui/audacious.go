// /home/krylon/go/src/github.com/blicero/raconteur/ui/audacious.go
// -*- mode: go; coding: utf-8; -*-
// Created on 08. 09. 2023 by Benjamin Walkenhorst
// (c) 2023 Benjamin Walkenhorst
// Time-stamp: <2023-09-08 13:20:34 krylon>

package ui

// We try to use audacious as our player. It offers a much richer DBus API than
// MPris2 does, so I'll try to use that to its fullest.

const (
	playerPath      = "org.mpris.MediaPlayer2.audacious"
	playerInterface = "org.atheme.audacious"
)

// /home/krylon/go/src/github.com/blicero/raconteur/ui/ui.go
// -*- mode: go; coding: utf-8; -*-
// Created on 12. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2021-09-18 18:32:04 krylon>

package ui

import (
	"log"

	"github.com/blicero/krylib"
	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/logdomain"
	"github.com/gotk3/gotk3/gtk"
)

// RWin is the user-facing part of the whole shebang.
type RWin struct {
	pool      *db.Pool
	log       *log.Logger
	win       *gtk.Window
	mainBox   *gtk.Box
	menu      *gtk.MenuBar
	statusbar *gtk.Statusbar
}

// Create creates a GUI. Who would've thought?
func Create() (*RWin, error) {
	var (
		err error
		win = new(RWin)
	)

	gtk.Init(nil)

	if win.log, err = common.GetLogger(logdomain.GUI); err != nil {
		return nil, err
	} else if win.pool, err = db.NewPool(4); err != nil {
		win.log.Printf("[ERROR] Cannot create database pool: %s\n",
			err.Error())
		return nil, err
	} else if win.win, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL); err != nil {
		win.log.Printf("[ERROR] Cannot create main window: %s\n",
			err.Error())
		return nil, err
	} else if win.mainBox, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL); err != nil {
		win.log.Printf("[ERROR] Cannot create main Box: %s\n",
			err.Error())
		return nil, err
	}

	return nil, krylib.ErrNotImplemented
} // func Create() (*RWin, error)

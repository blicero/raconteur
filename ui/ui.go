// /home/krylon/go/src/github.com/blicero/raconteur/ui/ui.go
// -*- mode: go; coding: utf-8; -*-
// Created on 12. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-05-31 23:18:21 krylon>

package ui

import (
	"fmt"
	"log"
	"time"

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
	lbl       *gtk.Label
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
	} else if win.mainBox, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 1); err != nil {
		win.log.Printf("[ERROR] Cannot create main Box: %s\n",
			err.Error())
		return nil, err
	} else if win.lbl, err = gtk.LabelNew("Wer das liest, ist doof"); err != nil {
		win.log.Printf("[ERROR] Cannot create label: %s\n",
			err.Error())
		return nil, err
	} else if win.statusbar, err = gtk.StatusbarNew(); err != nil {
		win.log.Printf("[ERROR] Cannot create status bar: %s\n", err.Error())
		return nil, err
	}

	win.win.Add(win.mainBox)
	win.mainBox.Add(win.lbl)
	win.mainBox.Add(win.statusbar)

	win.win.Connect("destroy", gtk.MainQuit)

	win.win.ShowAll()

	return win, nil
} // func Create() (*RWin, error)

// Run execute's gtk's main event loop.
func (w *RWin) Run() {
	go func() {
		var cnt = 0
		for {
			time.Sleep(time.Second)
			cnt += 1
			var msg = fmt.Sprintf("%s: Tick #%d",
				time.Now().Format(common.TimestampFormat),
				cnt)
			w.statusbar.Push(666, msg)
		}
	}()

	gtk.Main()
} // func (w *RWin) Run()

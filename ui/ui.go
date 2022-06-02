// /home/krylon/go/src/github.com/blicero/raconteur/ui/ui.go
// -*- mode: go; coding: utf-8; -*-
// Created on 12. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-02 21:57:20 krylon>

package ui

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/logdomain"
	"github.com/blicero/raconteur/objects"
	"github.com/blicero/raconteur/scanner"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// type tabContent struct {
// 	vbox   *gtk.Box
// 	sbox   *gtk.Box
// 	lbl    *gtk.Label
// 	search *gtk.Entry
// 	store  gtk.ITreeModel
// 	view   *gtk.TreeView
// 	scr    *gtk.ScrolledWindow
// }

// type cellEditHandlerFactory func(int) func(*gtk.CellRendererText, string, string)

type column struct {
	colType glib.Type
	title   string
	display bool
	edit    bool
}

var cols = []column{
	column{
		colType: glib.TYPE_INT,
		title:   "PID",
	},
	column{
		colType: glib.TYPE_STRING,
		title:   "Program",
		display: true,
	},
	column{
		colType: glib.TYPE_INT,
		title:   "ID",
	},
	column{
		colType: glib.TYPE_STRING,
		title:   "Title",
		edit:    true,
		display: true,
	},
	column{
		colType: glib.TYPE_INT,
		title:   "#",
		edit:    true,
		display: true,
	},
	column{
		colType: glib.TYPE_STRING,
		title:   "Dur",
		display: true,
	},
}

func createCol(title string, id int) (*gtk.TreeViewColumn, *gtk.CellRendererText, error) {
	krylib.Trace()
	defer fmt.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	renderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return nil, nil, err
	}

	col, err := gtk.TreeViewColumnNewWithAttribute(title, renderer, "text", id)
	if err != nil {
		return nil, nil, err
	}

	col.SetResizable(true)

	return col, renderer, nil
} // func createCol(title string, id int) (*gtk.TreeViewColumn, *gtk.CellRendererText, error)

// RWin is the user-facing part of the whole shebang.
// nolint: structcheck,unused
type RWin struct {
	pool        *db.Pool
	scanner     *scanner.Walker
	lock        sync.RWMutex
	log         *log.Logger
	win         *gtk.Window
	mainBox     *gtk.Box
	searchBox   *gtk.Box
	searchLbl   *gtk.Label
	searchEntry *gtk.Entry
	store       *gtk.TreeStore
	view        *gtk.TreeView
	scr         *gtk.ScrolledWindow
	menu        *gtk.MenuBar
	statusbar   *gtk.Statusbar
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
	} else if win.scanner, err = scanner.New(win.pool.Get()); err != nil {
		win.log.Printf("[ERROR] Cannot create Scanner: %s\n",
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
	} else if win.menu, err = gtk.MenuBarNew(); err != nil {
		win.log.Printf("[ERROR] Cannot create menu bar: %s\n",
			err.Error())
		return nil, err
	} else if win.searchBox, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 1); err != nil {
		win.log.Printf("[ERROR] Cannot create searchBox: %s\n",
			err.Error())
		return nil, err
	} else if win.searchLbl, err = gtk.LabelNew("Search: "); err != nil {
		win.log.Printf("[ERROR] Cannot create search label: %s\n",
			err.Error())
		return nil, err
	} else if win.searchEntry, err = gtk.EntryNew(); err != nil {
		win.log.Printf("[ERROR] Cannot create search entry: %s\n",
			err.Error())
		return nil, err
	} else if win.scr, err = gtk.ScrolledWindowNew(nil, nil); err != nil {
		win.log.Printf("[ERROR] Cannot create ScrolledWindow: %s\n",
			err.Error())
		return nil, err
	} else if win.statusbar, err = gtk.StatusbarNew(); err != nil {
		win.log.Printf("[ERROR] Cannot create status bar: %s\n", err.Error())
		return nil, err
	} // else if win.store, err = gtk.TreeStoreNew(

	var typeList = make([]glib.Type, len(cols))

	for i, c := range cols {
		typeList[i] = c.colType
	}

	if win.store, err = gtk.TreeStoreNew(typeList...); err != nil {
		win.log.Printf("[ERROR] Cannot create TreeStore: %s\n",
			err.Error())
		return nil, err
	} else if win.view, err = gtk.TreeViewNewWithModel(win.store); err != nil {
		win.log.Printf("[ERROR] Cannot create TreeView: %s\n",
			err.Error())
		return nil, err
	}

	for i, c := range cols {
		var (
			col      *gtk.TreeViewColumn
			renderer *gtk.CellRendererText
		)

		if col, renderer, err = createCol(c.title, i); err != nil {
			win.log.Printf("[ERROR] Cannot create TreeViewColumn %q: %s\n",
				c.title,
				err.Error())
			return nil, err
		}

		renderer.Set("editable", c.edit)     // nolint: errcheck
		renderer.Set("editable-set", c.edit) // nolint: errcheck

		win.view.AppendColumn(col)
	}

	win.searchBox.Add(win.searchLbl)
	win.searchBox.Add(win.searchEntry)
	win.win.Add(win.mainBox)
	win.scr.Add(win.view)
	win.mainBox.Add(win.menu)
	win.mainBox.Add(win.searchBox)
	win.mainBox.Add(win.scr)
	win.mainBox.Add(win.statusbar)

	if err = win.initializeTree(); err != nil {
		win.log.Printf("[ERROR] Failed to initialize TreeView: %s\n",
			err.Error())
		return nil, err
	} else if err = win.initializeMenu(); err != nil {
		win.log.Printf("[ERROR] Failed to initialize Menu: %s\n",
			err.Error())
		return nil, err
	}

	win.win.Connect("destroy", gtk.MainQuit)

	win.win.ShowAll()
	win.win.SetSizeRequest(960, 540)
	win.win.SetTitle(fmt.Sprintf("%s %s",
		common.AppName,
		common.Version))

	return win, nil
} // func Create() (*RWin, error)

func (w *RWin) initializeTree() error {
	var (
		err error
		d   = w.pool.Get()
	)
	defer w.pool.Put(d)

	var plist []objects.Program

	if plist, err = d.ProgramGetAll(); err != nil {
		w.log.Printf("[ERROR] Cannot load list of programs: %s\n",
			err.Error())
		return err
	}

	for _, p := range plist {
		var (
			flist []objects.File
			piter = w.store.Append(nil)
		)

		w.store.SetValue(piter, 0, p.ID)
		w.store.SetValue(piter, 1, p.Title)

		if flist, err = d.FileGetByProgram(&p); err != nil {
			w.log.Printf("[ERROR] Cannot get Files for Program %q: %s\n",
				p.Title,
				err.Error())
			return err
		}

		for _, f := range flist {
			var (
				dur   time.Duration
				fiter = w.store.Append(piter)
			)

			if dur, err = f.Duration(); err != nil {
				w.log.Printf("[ERROR] Cannot get Duration for File %q: %s\n",
					f.DisplayTitle(),
					err.Error())
				continue
			}

			w.store.SetValue(fiter, 2, f.ID)
			w.store.SetValue(fiter, 3, 0)
			w.store.SetValue(fiter, 4, dur.String())
		}
	}

	return nil
} // func (w *RWin) initializeTree() error

func (w *RWin) initializeMenu() error {
	var (
		err                error
		fileMenu, progMenu *gtk.Menu
		scanItem, quitItem *gtk.MenuItem
		progAddItem        *gtk.MenuItem
		fmItem, pmItem     *gtk.MenuItem
	)

	if fileMenu, err = gtk.MenuNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create File menu: %s\n",
			err.Error())
		return err
	} else if progMenu, err = gtk.MenuNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Program menu: %s\n",
			err.Error())
		return err
	} else if scanItem, err = gtk.MenuItemNewWithMnemonic("_Scan Folder"); err != nil {
		w.log.Printf("[ERROR] Cannot create menu item Scan: %s\n",
			err.Error())
		return err
	} else if quitItem, err = gtk.MenuItemNewWithMnemonic("_Quit"); err != nil {
		w.log.Printf("[ERROR] Cannot create menu item Quit: %s\n",
			err.Error())
		return err
	} else if progAddItem, err = gtk.MenuItemNewWithMnemonic("_Add"); err != nil {
		w.log.Printf("[ERROR] Cannot create menu item Add Program: %s\n",
			err.Error())
		return err
	} else if fmItem, err = gtk.MenuItemNewWithMnemonic("_File"); err != nil {
		w.log.Printf("[ERROR] Cannot create menu item File: %s\n",
			err.Error())
		return err
	} else if pmItem, err = gtk.MenuItemNewWithMnemonic("_Program"); err != nil {
		w.log.Printf("[ERROR] Cannot create menu item Program: %s\n",
			err.Error())
		return err
	}

	// Handlers!
	quitItem.Connect("activate", gtk.MainQuit)
	scanItem.Connect("activate", w.scanFolder)

	fileMenu.Append(scanItem)
	fileMenu.Append(quitItem)

	progMenu.Append(progAddItem)

	w.menu.Append(fmItem)
	w.menu.Append(pmItem)

	fmItem.SetSubmenu(fileMenu)
	pmItem.SetSubmenu(progMenu)

	return nil
} // func (w *RWin) initializeMenu() error

// Run execute's gtk's main event loop.
func (w *RWin) Run() {
	go func() {
		var cnt = 0
		for {
			time.Sleep(time.Second)
			cnt++
			var msg = fmt.Sprintf("%s: Tick #%d",
				time.Now().Format(common.TimestampFormat),
				cnt)
			w.statusbar.Push(666, msg)
		}
	}()

	gtk.Main()
} // func (w *RWin) Run()

// nolint: unused
func (w *RWin) displayMsg(msg string) {
	krylib.Trace()
	defer w.log.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	var (
		err error
		dlg *gtk.Dialog
		lbl *gtk.Label
		box *gtk.Box
	)

	if dlg, err = gtk.DialogNewWithButtons(
		"Message",
		w.win,
		gtk.DIALOG_MODAL,
		[]interface{}{
			"Okay",
			gtk.RESPONSE_OK,
		},
	); err != nil {
		w.log.Printf("[ERROR] Cannot create dialog to display message: %s\nMesage would've been %q\n",
			err.Error(),
			msg)
		return
	}

	defer dlg.Close()

	if lbl, err = gtk.LabelNew(msg); err != nil {
		w.log.Printf("[ERROR] Cannot create label to display message: %s\nMessage would've been: %q\n",
			err.Error(),
			msg)
		return
	} else if box, err = dlg.GetContentArea(); err != nil {
		w.log.Printf("[ERROR] Cannot get ContentArea of Dialog to display message: %s\nMessage would've been %q\n",
			err.Error(),
			msg)
		return
	}

	box.PackStart(lbl, true, true, 0)
	dlg.ShowAll()
	dlg.Run()
} // func (w *RWin) displayMsg(msg string)

func (w *RWin) scanFolder() {
	krylib.Trace()
	defer w.log.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	var (
		err error
		dlg *gtk.FileChooserDialog
		res gtk.ResponseType
	)

	if dlg, err = gtk.FileChooserDialogNewWith2Buttons(
		"Scan Folder",
		w.win,
		gtk.FILE_CHOOSER_ACTION_SELECT_FOLDER,
		"Cancel",
		gtk.RESPONSE_CANCEL,
		"OK",
		gtk.RESPONSE_OK,
	); err != nil {
		w.log.Printf("[ERROR] Cannot create FileChooserDialog: %s\n",
			err.Error())
		return
	}

	defer dlg.Close()

	res = dlg.Run()

	switch res {
	case gtk.RESPONSE_CANCEL:
		w.log.Println("[DEBUG] Ha, you almost got me.")
		return
	case gtk.RESPONSE_OK:
		var path string
		if path, err = dlg.GetCurrentFolder(); err != nil {
			w.log.Printf("[ERROR] Cannot get folder from dialog: %s\n",
				err.Error())
			return
		}

		go w.scanner.Walk(path)
	}
} // func (w *RWin) scanFolder()

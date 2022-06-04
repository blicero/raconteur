// /home/krylon/go/src/github.com/blicero/raconteur/ui/ui.go
// -*- mode: go; coding: utf-8; -*-
// Created on 12. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-04 21:21:11 krylon>

package ui

import (
	"fmt"
	"log"
	"math"
	"net/url"
	"sync"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/logdomain"
	"github.com/blicero/raconteur/objects"
	"github.com/blicero/raconteur/scanner"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

const (
	ckFileInterval = time.Millisecond * 25
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
	ticker      *time.Ticker
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
	win.mainBox.PackStart(win.menu, false, false, 1)
	win.mainBox.PackStart(win.searchBox, false, false, 1)
	win.mainBox.PackStart(win.scr, true, true, 1)
	win.mainBox.PackStart(win.statusbar, false, false, 1)

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

	win.view.Connect("button-press-event", win.handleFileListClick)

	win.ticker = time.NewTicker(ckFileInterval)
	glib.IdleAdd(win.ckFileQueue)

	win.win.ShowAll()
	win.win.SetSizeRequest(960, 540)
	win.win.SetTitle(fmt.Sprintf("%s %s",
		common.AppName,
		common.Version))

	return win, nil
} // func Create() (*RWin, error)

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

			w.store.SetValue(fiter, 0, -f.ProgramID)
			w.store.SetValue(fiter, 2, f.ID)
			w.store.SetValue(fiter, 3, f.DisplayTitle())
			w.store.SetValue(fiter, 4, 0)
			w.store.SetValue(fiter, 5, dur.String())
		}
	}

	var (
		flist []objects.File
		piter *gtk.TreeIter
	)

	if flist, err = d.FileGetNoProgram(); err != nil {
		w.log.Printf("[ERROR] Cannot get Files without an assigned Program: %s\n",
			err.Error())
		return err
	}

	piter = w.store.Prepend(nil)
	w.store.SetValue(piter, 0, 0)
	w.store.SetValue(piter, 1, "---")

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

		w.store.SetValue(fiter, 0, math.MinInt32)
		w.store.SetValue(fiter, 2, f.ID)
		w.store.SetValue(fiter, 3, f.DisplayTitle())
		w.store.SetValue(fiter, 4, 0)
		w.store.SetValue(fiter, 5, dur.String())
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
	progAddItem.Connect("activate", w.handleAddProgram)

	fileMenu.Append(scanItem)
	fileMenu.Append(quitItem)

	progMenu.Append(progAddItem)

	w.menu.Append(fmItem)
	w.menu.Append(pmItem)

	fmItem.SetSubmenu(fileMenu)
	pmItem.SetSubmenu(progMenu)

	return nil
} // func (w *RWin) initializeMenu() error

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

func (w *RWin) handleAddProgram() {
	var (
		err                    error
		dlg                    *gtk.Dialog
		p                      objects.Program
		s                      string
		dbox                   *gtk.Box
		grid                   *gtk.Grid
		uLbl, tLbl, cLbl       *gtk.Label
		uEntry, tEntry, cEntry *gtk.Entry
	)

	if dlg, err = gtk.DialogNewWithButtons(
		"Add Program",
		w.win,
		gtk.DIALOG_MODAL,
		[]any{
			"_Cancel",
			gtk.RESPONSE_CANCEL,
			"_OK",
			gtk.RESPONSE_OK,
		}); err != nil {
		w.log.Printf("[ERROR] Cannot create dialog for adding Program: %s\n",
			err.Error())
		return
	}

	defer dlg.Close()

	if _, err = dlg.AddButton("OK", gtk.RESPONSE_OK); err != nil {
		w.log.Printf("[ERROR] Cannot add OK button to AddProgram Dialog: %s\n",
			err.Error())
		return
	} else if grid, err = gtk.GridNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create gtk.Grid for AddPerson Dialog: %s\n",
			err.Error())
		return
	} else if uLbl, err = gtk.LabelNew("URL:"); err != nil {
		w.log.Printf("[ERROR] Cannot create URL Label: %s\n",
			err.Error())
		return
	} else if tLbl, err = gtk.LabelNew("Title:"); err != nil {
		w.log.Printf("[ERROR] Cannot create Title Label: %s\n",
			err.Error())
		return
	} else if cLbl, err = gtk.LabelNew("Creator:"); err != nil {
		w.log.Printf("[ERROR] Cannot create Creator Label: %s\n",
			err.Error())
		return
	} else if uEntry, err = gtk.EntryNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Entry for URL: %s\n",
			err.Error())
		return
	} else if tEntry, err = gtk.EntryNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Entry for Title: %s\n",
			err.Error())
		return
	} else if cEntry, err = gtk.EntryNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Entry for Creator: %s\n",
			err.Error())
		return
	} else if dbox, err = dlg.GetContentArea(); err != nil {
		w.log.Printf("[ERROR] Cannot get ContentArea of AddPerson Dialog: %s\n",
			err.Error())
		return
	}

	grid.InsertColumn(0)
	grid.InsertColumn(1)
	grid.InsertRow(0)
	grid.InsertRow(1)
	grid.InsertRow(2)

	grid.Attach(tLbl, 0, 0, 1, 1)
	grid.Attach(uLbl, 0, 1, 1, 1)
	grid.Attach(cLbl, 0, 2, 1, 1)
	grid.Attach(tEntry, 1, 0, 1, 1)
	grid.Attach(uEntry, 1, 1, 1, 1)
	grid.Attach(cEntry, 1, 2, 1, 1)

	dbox.PackStart(grid, true, true, 0)
	dlg.ShowAll()

	var res = dlg.Run()

	switch res {
	case gtk.RESPONSE_NONE:
		fallthrough
	case gtk.RESPONSE_DELETE_EVENT:
		fallthrough
	case gtk.RESPONSE_CLOSE:
		fallthrough
	case gtk.RESPONSE_CANCEL:
		w.log.Println("[DEBUG] User changed their mind about adding a Program. Fine with me.")
		return
	case gtk.RESPONSE_OK:
		// 's ist los, Hund?
	default:
		w.log.Printf("[CANTHAPPEN] Well, I did NOT see this coming: %d\n",
			res)
		return
	}

	if p.Title, err = tEntry.GetText(); err != nil {
		w.log.Printf("[ERROR] Cannot get Title: %s\n",
			err.Error())
		return
	} else if p.Creator, err = cEntry.GetText(); err != nil {
		w.log.Printf("[ERROR] Cannot get Creator: %s\n",
			err.Error())
		return
	} else if s, err = uEntry.GetText(); err != nil {
		w.log.Printf("[ERROR] Cannot get URL: %s\n",
			err.Error())
		return
	} else if p.URL, err = url.Parse(s); err != nil {
		w.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
			s,
			err.Error())
		return
	}

	var d *db.Database

	d = w.pool.Get()
	defer w.pool.Put(d)

	if err = d.ProgramAdd(&p); err != nil {
		w.log.Printf("[ERROR] Cannot add Program %q: %s\n",
			p.Title,
			err.Error())
		return
	}

	var piter = w.store.Append(nil)

	w.store.SetValue(piter, 0, p.ID)
	w.store.SetValue(piter, 1, p.Title)
} // func (w *RWin) handleAddProgram()

func (w *RWin) ckFileQueue() bool {
	var f *objects.File

	// w.log.Printf("[TRACE] Checking file queue\n")

	select {
	case <-w.ticker.C:
		// do nothing
	case f = <-w.scanner.Files:
		var (
			err          error
			dur          time.Duration
			dstr         string
			piter, fiter *gtk.TreeIter
		)

		w.log.Printf("[DEBUG] Got file from queue: %s\n", f.Path)

		if dur, err = f.Duration(); err != nil {
			w.log.Printf("[ERROR] Cannot determine duration of file %s: %s\n",
				f.Path,
				err.Error())
			dstr = "N/A"
		} else {
			dstr = dur.String()
		}

		piter, _ = w.store.GetIterFirst()

		fiter = w.store.Append(piter)

		w.store.SetValue(fiter, 0, -1)
		w.store.SetValue(fiter, 2, f.ID)
		w.store.SetValue(fiter, 3, f.DisplayTitle())
		w.store.SetValue(fiter, 4, 0)
		w.store.SetValue(fiter, 5, dstr)
	}

	return true
} // func (w *RWin) ckFileQueue()

func (w *RWin) handleFileListClick(view *gtk.TreeView, evt *gdk.Event) {
	krylib.Trace()
	defer w.log.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())
	var be = gdk.EventButtonNewFromEvent(evt)

	if be.Button() != gdk.BUTTON_SECONDARY {
		return
	}

	var (
		err    error
		exists bool
		x, y   float64
		path   *gtk.TreePath
		col    *gtk.TreeViewColumn
		model  *gtk.TreeModel
		imodel gtk.ITreeModel
		iter   *gtk.TreeIter
	)

	x = be.X()
	y = be.Y()

	path, col, _, _, exists = view.GetPathAtPos(int(x), int(y))

	if !exists {
		w.log.Printf("[DEBUG] There is no item at %f/%f\n",
			x,
			y)
		return
	}

	w.log.Printf("[DEBUG] Handle Click at %f/%f -> Path %s\n",
		x,
		y,
		path)

	if imodel, err = view.GetModel(); err != nil {
		w.log.Printf("[ERROR] Cannot get Model from View: %s\n",
			err.Error())
		return
	}

	model = imodel.ToTreeModel()

	if iter, err = model.GetIter(path); err != nil {
		w.log.Printf("[ERROR] Cannot get Iter from TreePath %s: %s\n",
			path,
			err.Error())
		return
	}

	var title string = col.GetTitle()
	w.log.Printf("[DEBUG] Column %s was clicked\n",
		title)

	var (
		val      *glib.Value
		gv       interface{}
		pid, fid int64
	)

	if val, err = model.GetValue(iter, 0); err != nil {
		w.log.Printf("[ERROR] Cannot get value for column 0: %s\n",
			err.Error())
		return
	} else if gv, err = val.GoValue(); err != nil {
		w.log.Printf("[ERROR] Cannot get Go value from GLib value: %s\n",
			err.Error())
	}

	switch v := gv.(type) {
	case int:
		pid = int64(v)
	case int64:
		pid = v
	default:
		w.log.Printf("[ERROR] Unexpected type for ID column: %T\n",
			v)
		return
	}

	if val, err = model.GetValue(iter, 2); err != nil {
		w.log.Printf("[ERROR] Cannot get value for column 2: %s\n",
			err.Error())
		return
	} else if gv, err = val.GoValue(); err != nil {
		w.log.Printf("[ERROR] Cannot get value from GLib value: %s\n",
			err.Error())
		return
	}

	switch v := gv.(type) {
	case int:
		fid = int64(v)
	case int64:
		fid = v
	default:
		w.log.Printf("[ERROR] Unexpected type for ID column: %T\n",
			v)
		return
	}

	w.log.Printf("[DEBUG] PID of clicked-on row is %d, FID is %d\n",
		pid,
		fid)

	// First of all, we need to figure out if we clicked on a Program or a File.
	if pid >= 0 {
		w.log.Printf("[DEBUG] We clicked on a Program\n")
	} else {
		w.log.Printf("[DEBUG] We clicked on a File\n")
	}
} // func (w *RWin) handleFileListClick(view *gtk.TreeView, evt *gdk.Event)

// /home/krylon/go/src/github.com/blicero/raconteur/ui/ui.go
// -*- mode: go; coding: utf-8; -*-
// Created on 12. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-28 20:05:25 krylon>

package ui

import (
	"fmt"
	"log"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/logdomain"
	"github.com/blicero/raconteur/objects"
	"github.com/blicero/raconteur/scanner"
	"github.com/godbus/dbus/v5"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// rescanInterval is set to a very low value intentionally for purposes
// of development/testing/debugging. Once I've done that, I should set it
// to a more reasonable value, say 30 or 60 minutes.
const (
	ckFileInterval = time.Millisecond * 25
	rescanInterval = time.Second * 30
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
	pool         *db.Pool
	scanner      *scanner.Walker
	ticker       *time.Ticker
	lock         sync.RWMutex
	log          *log.Logger
	win          *gtk.Window
	mainBox      *gtk.Box
	searchBox    *gtk.Box
	searchLbl    *gtk.Label
	searchEntry  *gtk.Entry
	store        *gtk.TreeStore
	view         *gtk.TreeView
	scr          *gtk.ScrolledWindow
	menu         *gtk.MenuBar
	dMenu        *gtk.Menu
	statusbar    *gtk.Statusbar
	progs        map[int64]*gtk.TreeIter
	fCache       map[int64]*objects.File
	pCache       map[int64]*objects.Program
	dCache       map[int64]*objects.Folder
	mbus         *dbus.Conn
	sigq         chan *dbus.Signal
	playerActive bool
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
	} else if win.mbus, err = dbus.SessionBus(); err != nil {
		win.log.Printf("[ERROR] Cannot connect to session bus: %s\n",
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
	}

	win.fCache = make(map[int64]*objects.File)
	win.pCache = make(map[int64]*objects.Program)
	win.dCache = make(map[int64]*objects.Folder)
	win.sigq = make(chan *dbus.Signal, 25)

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

	win.store.SetDefaultSortFunc(win.cmpIter)

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
	glib.TimeoutAdd(uint(rescanInterval.Milliseconds()), win.refreshFolders)

	go func() {
		var (
			ex error
			s  string
		)

		for {
			time.Sleep(time.Second * 5)
			if s, ex = win.getPlayerStatus(); ex != nil {
				// win.log.Printf("[ERROR] Cannot query player status: %s\n",
				// 	ex.Error())
			} else {
				win.log.Printf("[DEBUG] Player is %s\n", s)
			}

		}
	}()

	win.win.ShowAll()
	win.win.SetSizeRequest(960, 540)
	win.win.SetTitle(fmt.Sprintf("%s %s",
		common.AppName,
		common.Version))

	return win, nil
} // func Create() (*RWin, error)

// Run executes gtk's main event loop.
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
		sel *gtk.TreeSelection
		d   = w.pool.Get()
	)
	defer w.pool.Put(d)

	if sel, err = w.view.GetSelection(); err != nil {
		w.log.Printf("[ERROR] Cannot get TreeSelection: %s\n",
			err.Error())
		return err
	}

	//sel.SetMode(gtk.SELECTION_MULTIPLE)
	sel.SetMode(gtk.SELECTION_SINGLE)

	var (
		plist []objects.Program
		dlist []objects.Folder
	)

	if plist, err = d.ProgramGetAll(); err != nil {
		w.log.Printf("[ERROR] Cannot load list of programs: %s\n",
			err.Error())
		return err
	} else if dlist, err = d.FolderGetAll(); err != nil {
		w.log.Printf("[ERROR] Cannot load list of Folders: %s\n",
			err.Error())
		return err
	}

	for _, f := range dlist {
		w.dCache[f.ID] = f.Clone()
	}

	w.progs = make(map[int64]*gtk.TreeIter)

	for _, p := range plist {
		var (
			flist []objects.File
			piter = w.store.Append(nil)
		)

		w.pCache[p.ID] = p.Clone()
		w.progs[p.ID] = piter
		w.store.SetValue(piter, 0, p.ID)    // nolint: errcheck
		w.store.SetValue(piter, 1, p.Title) // nolint: errcheck

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

			w.fCache[f.ID] = f.Clone()

			if dur, err = f.Duration(); err != nil {
				w.log.Printf("[ERROR] Cannot get Duration for File %q: %s\n",
					f.DisplayTitle(),
					err.Error())
				continue
			}

			w.store.SetValue(fiter, 0, -f.ProgramID)     // nolint: errcheck
			w.store.SetValue(fiter, 2, f.ID)             // nolint: errcheck
			w.store.SetValue(fiter, 3, f.DisplayTitle()) // nolint: errcheck
			w.store.SetValue(fiter, 4, f.Ord[1])         // nolint: errcheck
			w.store.SetValue(fiter, 5, dur.String())     // nolint: errcheck
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
	w.progs[0] = piter
	w.store.SetValue(piter, 0, 0)     // nolint: errcheck
	w.store.SetValue(piter, 1, "---") // nolint: errcheck

	for _, f := range flist {
		var (
			dur   time.Duration
			fiter = w.store.Append(piter)
		)

		w.fCache[f.ID] = f.Clone()

		if dur, err = f.Duration(); err != nil {
			w.log.Printf("[ERROR] Cannot get Duration for File %q: %s\n",
				f.DisplayTitle(),
				err.Error())
			continue
		}

		w.store.SetValue(fiter, 0, math.MinInt32)    // nolint: errcheck
		w.store.SetValue(fiter, 2, f.ID)             // nolint: errcheck
		w.store.SetValue(fiter, 3, f.DisplayTitle()) // nolint: errcheck
		w.store.SetValue(fiter, 4, 0)                // nolint: errcheck
		w.store.SetValue(fiter, 5, dur.String())     // nolint: errcheck
	}

	return nil
} // func (w *RWin) initializeTree() error

func (w *RWin) initializeMenu() error {
	var (
		err                            error
		fileMenu, progMenu, folderMenu *gtk.Menu
		scanItem, quitItem             *gtk.MenuItem
		progAddItem                    *gtk.MenuItem
		fmItem, pmItem, dmItem         *gtk.MenuItem
	)

	if fileMenu, err = gtk.MenuNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create File menu: %s\n",
			err.Error())
		return err
	} else if progMenu, err = gtk.MenuNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Program menu: %s\n",
			err.Error())
		return err
	} else if folderMenu, err = gtk.MenuNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Folder menu: %s\n",
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
	} else if dmItem, err = gtk.MenuItemNewWithMnemonic("_Directories"); err != nil {
		w.log.Printf("[ERROR] Cannot create menu item Directories: %s\n",
			err.Error())
		return err
	}

	var (
		conn    *db.Database
		folders []objects.Folder
	)

	conn = w.pool.Get()
	defer w.pool.Put(conn)

	if folders, err = conn.FolderGetAll(); err != nil {
		w.log.Printf("[ERROR] Cannot load all Folders: %s\n",
			err.Error())
		return err
	}

	for _, f := range folders {
		var item *gtk.MenuItem

		if item, err = gtk.MenuItemNewWithLabel(f.Path); err != nil {
			w.log.Printf("[ERROR] Cannot create MenuItem for %q: %s\n",
				f.Path,
				err.Error())
			return err
		}

		folderMenu.Append(item)

		item.Connect("activate", func() {
			w.statusbar.Push(666, fmt.Sprintf("Update %s", f.Path))
			go w.scanner.Walk(f.Path) // nolint: errcheck
		})
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
	w.menu.Append(dmItem)

	fmItem.SetSubmenu(fileMenu)
	pmItem.SetSubmenu(progMenu)
	dmItem.SetSubmenu(folderMenu)
	w.dMenu = folderMenu

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

		go w.scanner.Walk(path) // nolint: errcheck
		glib.TimeoutAdd(1000,
			func() bool {
				var (
					ex   error
					item *gtk.MenuItem
				)

				if item, ex = gtk.MenuItemNewWithLabel(path); ex != nil {
					w.log.Printf("[ERROR] Cannot create MenuItem for %q: %s\n",
						path,
						ex.Error())
					return false
				}

				item.Connect("activate", func() {
					w.statusbar.Push(666, fmt.Sprintf("Update %s", path))
					go w.scanner.Walk(path) // nolint: errcheck
				})

				w.dMenu.Append(item)

				return false
			})
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

	var d = w.pool.Get()
	defer w.pool.Put(d)

	if err = d.ProgramAdd(&p); err != nil {
		w.log.Printf("[ERROR] Cannot add Program %q: %s\n",
			p.Title,
			err.Error())
		return
	}

	w.pCache[p.ID] = p.Clone()

	var piter = w.store.Append(nil)

	w.progs[p.ID] = piter

	w.store.SetValue(piter, 0, p.ID)    // nolint: errcheck
	w.store.SetValue(piter, 1, p.Title) // nolint: errcheck
} // func (w *RWin) handleAddProgram()

func (w *RWin) ckFileQueue() bool {
	var f *objects.File

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

		if _, ok := w.fCache[f.ID]; ok {
			return true
		}

		w.fCache[f.ID] = f

		if dur, err = f.Duration(); err != nil {
			w.log.Printf("[ERROR] Cannot determine duration of file %s: %s\n",
				f.Path,
				err.Error())
			dstr = "N/A"
		} else {
			dstr = dur.String()
		}

		if f.ProgramID != 0 {
			if piter = w.progs[f.ProgramID]; piter == nil {
				var (
					p *objects.Program
					c = w.pool.Get()
				)
				defer w.pool.Put(c)

				if p, err = c.ProgramGetByID(f.ProgramID); err != nil {
					w.log.Printf("[ERROR] Cannot fetch Program %d from database: %s\n",
						f.ProgramID,
						err.Error())
					piter, _ = w.store.GetIterFirst()
				} else if p == nil {
					w.log.Printf("[CANTHAPPEN] Cannot find Program %d in database.\n",
						f.ProgramID)
					piter, _ = w.store.GetIterFirst()
				} else {
					piter = w.store.Append(nil)
					w.store.SetValue(piter, 0, p.ID)    // nolint: errcheck
					w.store.SetValue(piter, 1, p.Title) // nolint: errcheck
					w.progs[p.ID] = piter
				}
			}
		} else {
			piter, _ = w.store.GetIterFirst()
		}

		fiter = w.store.Append(piter)

		w.store.SetValue(fiter, 0, math.MinInt32)    // nolint: errcheck
		w.store.SetValue(fiter, 2, f.ID)             // nolint: errcheck
		w.store.SetValue(fiter, 3, f.DisplayTitle()) // nolint: errcheck
		w.store.SetValue(fiter, 4, f.Ord[1])         // nolint: errcheck
		w.store.SetValue(fiter, 5, dstr)             // nolint: errcheck
	}

	return true
} // func (w *RWin) ckFileQueue()

func (w *RWin) handleFileListClick(view *gtk.TreeView, evt *gdk.Event) {
	var be = gdk.EventButtonNewFromEvent(evt)

	// We are only interested in right-clicks.
	if be.Button() != gdk.BUTTON_SECONDARY {
		return
	}

	var (
		err    error
		exists bool
		x, y   float64
		path   *gtk.TreePath
		col    *gtk.TreeViewColumn
		iter   *gtk.TreeIter
		iters  []*gtk.TreeIter
		sel    *gtk.TreeSelection
		rows   *glib.List
		menu   *gtk.Menu
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

	if sel, err = w.view.GetSelection(); err != nil {
		w.log.Printf("[ERROR] Cannot get TreeSelection: %s\n",
			err.Error())
		return
	}

	rows = sel.GetSelectedRows(w.store)

	iters = make([]*gtk.TreeIter, 0, rows.Length())

	rows.Foreach(func(i interface{}) {
		switch r := i.(type) {
		case *gtk.TreePath:
			if i, e := w.store.GetIter(r); err != nil {
				w.log.Printf("[ERROR] Cannot get Iter for Path %s: %s\n",
					r,
					e.Error())
			} else {
				iters = append(iters, i)
			}
		default:
			w.log.Printf("[CANTHAPPEN] Invalid type for GetSelectedRows: %T\n",
				i)
		}
	})

	if len(iters) == 0 {
		return
	} else if len(iters) == 1 {
		iter = iters[0]

		var title string = col.GetTitle()
		w.log.Printf("[DEBUG] Column %s was clicked\n",
			title)

		var (
			pid, fid int64
		)

		if pid, fid, err = w.getPidFid(iter); err != nil {
			w.log.Printf("[ERROR] Cannot get PID/FID: %s\n",
				err.Error())
			return
		}

		w.log.Printf("[DEBUG] PID of clicked-on row is %d, FID is %d\n",
			pid,
			fid)

		// First of all, we need to figure out if we clicked on a Program or a File.
		if pid >= 0 {
			w.log.Printf("[DEBUG] We clicked on a Program\n")
			if menu, err = w.mkContextMenuProgram(iter, pid); err != nil {
				w.log.Printf("[ERROR] Cannot create context menu: %s\n",
					err.Error())
				return
			}
		} else {
			w.log.Printf("[DEBUG] We clicked on a File\n")
			if menu, err = w.mkContextMenuFile(iter, fid); err != nil {
				w.log.Printf("[ERROR] Cannot create context menu: %s\n",
					err.Error())
				return
			}
		}
	} else {

		w.displayMsg("Handling multiple selection is not implemented, yet.")
		return
	}

	menu.ShowAll()
	menu.PopupAtPointer(evt)
} // func (w *RWin) handleFileListClick(view *gtk.TreeView, evt *gdk.Event)

func (w *RWin) mkContextMenuProgram(iter *gtk.TreeIter, pid int64) (*gtk.Menu, error) {
	var (
		err                error
		editItem, playItem *gtk.MenuItem
		menu               *gtk.Menu
		c                  *db.Database
		p                  *objects.Program
	)

	w.log.Printf("[TRACE] Make Context Menu for Program %d\n", pid)

	c = w.pool.Get()
	defer w.pool.Put(c)

	if p, err = c.ProgramGetByID(pid); err != nil {
		w.log.Printf("[ERROR] Cannot load Program %d: %s\n",
			pid,
			err.Error())
		return nil, err
	} else if p == nil {
		w.log.Printf("[ERROR] Cannot find Program #%d in database!\n", pid)
		return nil, fmt.Errorf("Program ID %d was not found in database", pid)
	} else if menu, err = gtk.MenuNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Menu: %s\n",
			err.Error())
		return nil, err
	} else if editItem, err = gtk.MenuItemNewWithMnemonic("_Edit"); err != nil {
		w.log.Printf("[ERROR] Cannot create MenuItem \"Edit\": %s\n",
			err.Error())
		return nil, err
	} else if playItem, err = gtk.MenuItemNewWithMnemonic("_Play"); err != nil {
		w.log.Printf("[ERROR] Cannot create MenuItem \"Play\": %s\n",
			err.Error())
		return nil, err
	}

	editItem.Connect("activate", func() { w.editProgram(p, iter) })
	playItem.Connect("activate", func() { w.playProgram(p) })

	menu.Append(editItem)
	menu.Append(playItem)

	return menu, nil
} // func (w *RWin) mkContextMenuProgram(iter *gtk.TreeIter, pid int64) (*gtk.Menu, error)

func (w *RWin) mkContextMenuFile(iter *gtk.TreeIter, fid int64) (*gtk.Menu, error) {
	var (
		err                error
		editItem, progItem *gtk.MenuItem
		menu, progMenu     *gtk.Menu
		c                  *db.Database
		f                  *objects.File
		progs              []objects.Program
	)

	c = w.pool.Get()
	defer w.pool.Put(c)

	if f, err = c.FileGetByID(fid); err != nil {
		w.log.Printf("[ERROR] Cannot load File %d: %s\n",
			fid,
			err.Error())
		return nil, err
	} else if progs, err = c.ProgramGetAll(); err != nil {
		w.log.Printf("[ERROR] Cannot load all Programs: %s\n",
			err.Error())
		return nil, err
	} else if menu, err = gtk.MenuNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create context menu: %s\n",
			err.Error())
		return nil, err
	} else if progMenu, err = gtk.MenuNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Program submenu: %s\n",
			err.Error())
		return nil, err
	} else if editItem, err = gtk.MenuItemNewWithMnemonic("_Edit"); err != nil {
		w.log.Printf("[ERROR] Cannot create Edit item: %s\n",
			err.Error())
		return nil, err
	} else if progItem, err = gtk.MenuItemNewWithMnemonic("_Program"); err != nil {
		w.log.Printf("[ERROR] Cannot create Program item: %s\n",
			err.Error())
		return nil, err
	}

	editItem.Connect("activate", func() { w.editFile(f, iter) })
	menu.Append(editItem)
	menu.Append(progItem)
	progItem.SetSubmenu(progMenu)

	var (
		pitem *gtk.CheckMenuItem
	)

	if pitem, err = gtk.CheckMenuItemNewWithLabel("(NULL)"); err != nil {
		w.log.Printf("[ERROR] Cannot create CheckMenuItem: %s\n",
			err.Error())
		return nil, err
	}

	pitem.SetActive(f.ProgramID == 0)
	pitem.Connect("activate", func() { w.fileSetProgram(iter, f, nil) })
	progMenu.Append(pitem)

	for _, p := range progs {
		if pitem, err = gtk.CheckMenuItemNewWithLabel(p.Title); err != nil {
			w.log.Printf("[ERROR] Cannot create RadioMenuItem for Program %q: %s\n",
				p.Title,
				err.Error())
			return nil, err
		}

		w.log.Printf("[TRACE] Create menu item for Program %d (%q)\n",
			p.ID,
			p.Title)

		var pParm = p.Clone()

		pitem.SetActive(p.ID == f.ProgramID)
		pitem.Connect("activate", func() { w.fileSetProgram(iter, f, pParm) })
		progMenu.Append(pitem)
	}

	return menu, nil
} // func (w *RWin) mkContextMenuFile(iter *gtk.TreeIter, fid int64) (*gtk.Menu, error)

// func (w *RWin) mkContextMenuMultipleFiles(iters []*gtk.TreeIter, fids []int64) (*gtk.Menu, error) {
// 	var (
// 		err                error
// 		editItem, progItem *gtk.MenuItem
// 		menu, progMenu     *gtk.Menu
// 		c                  *db.Database
// 		f                  *objects.File
// 		progs              []objects.Program
// 	)

// 	c = w.pool.Get()
// 	defer w.pool.Put(c)

// 	if f, err = c.FileGetByID(fid); err != nil {
// 		w.log.Printf("[ERROR] Cannot load File %d: %s\n",
// 			fid,
// 			err.Error())
// 		return nil, err
// 	} else if progs, err = c.ProgramGetAll(); err != nil {
// 		w.log.Printf("[ERROR] Cannot load all Programs: %s\n",
// 			err.Error())
// 		return nil, err
// 	} else if menu, err = gtk.MenuNew(); err != nil {
// 		w.log.Printf("[ERROR] Cannot create context menu: %s\n",
// 			err.Error())
// 		return nil, err
// 	} else if progMenu, err = gtk.MenuNew(); err != nil {
// 		w.log.Printf("[ERROR] Cannot create Program submenu: %s\n",
// 			err.Error())
// 		return nil, err
// 	} else if editItem, err = gtk.MenuItemNewWithMnemonic("_Edit"); err != nil {
// 		w.log.Printf("[ERROR] Cannot create Edit item: %s\n",
// 			err.Error())
// 		return nil, err
// 	} else if progItem, err = gtk.MenuItemNewWithMnemonic("_Program"); err != nil {
// 		w.log.Printf("[ERROR] Cannot create Program item: %s\n",
// 			err.Error())
// 		return nil, err
// 	}

// 	editItem.Connect("activate", func() { w.editFile(f, iter) })
// 	menu.Append(editItem)
// 	menu.Append(progItem)
// 	progItem.SetSubmenu(progMenu)

// 	var (
// 		pitem *gtk.CheckMenuItem
// 	)

// 	if pitem, err = gtk.CheckMenuItemNewWithLabel("(NULL)"); err != nil {
// 		w.log.Printf("[ERROR] Cannot create CheckMenuItem: %s\n",
// 			err.Error())
// 		return nil, err
// 	}

// 	pitem.SetActive(f.ProgramID == 0)
// 	pitem.Connect("activate", func() { w.fileSetProgram(iter, f, nil) })
// 	progMenu.Append(pitem)

// 	for _, p := range progs {
// 		if pitem, err = gtk.CheckMenuItemNewWithLabel(p.Title); err != nil {
// 			w.log.Printf("[ERROR] Cannot create RadioMenuItem for Program %q: %s\n",
// 				p.Title,
// 				err.Error())
// 			return nil, err
// 		}

// 		w.log.Printf("[TRACE] Create menu item for Program %d (%q)\n",
// 			p.ID,
// 			p.Title)

// 		var pParm = p.Clone()

// 		pitem.SetActive(p.ID == f.ProgramID)
// 		pitem.Connect("activate", func() { w.fileSetProgram(iter, f, pParm) })
// 		progMenu.Append(pitem)
// 	}

// 	return menu, nil
// } // func (w *RWin) mkContextMenu(iter *gtk.TreeIter, fid int64) (*gtk.Menu, error)

func (w *RWin) fileSetProgram(iter *gtk.TreeIter, f *objects.File, p *objects.Program) {
	var (
		err          error
		c            *db.Database
		fiter, piter *gtk.TreeIter
		dstr         string
		dur          time.Duration
		pid          int64
	)

	if p != nil {
		pid = p.ID
	}

	if f.ProgramID == pid {
		w.log.Printf("[DEBUG] ProgramID of File %d (%d) is not changed.\n",
			f.ID,
			pid)
		return
	}

	c = w.pool.Get()
	defer w.pool.Put(c)

	if err = c.FileSetProgram(f, p); err != nil {
		w.log.Printf("[ERROR] Cannot update Program of File %q to %q: %s\n",
			f.DisplayTitle(),
			p.Title,
			err.Error())
		return
	}

	piter = w.progs[pid]

	fiter = w.store.Append(piter)

	if dur, err = f.Duration(); err != nil {
		w.log.Printf("[ERROR] Cannot determine duration of file %s: %s\n",
			f.Path,
			err.Error())
		dstr = "N/A"
	} else {
		dstr = dur.String()
	}

	if pid == 0 {
		pid = math.MinInt32
	}

	w.store.SetValue(fiter, 0, pid)              // nolint: errcheck
	w.store.SetValue(fiter, 2, f.ID)             // nolint: errcheck
	w.store.SetValue(fiter, 3, f.DisplayTitle()) // nolint: errcheck
	w.store.SetValue(fiter, 4, 0)                // nolint: errcheck
	w.store.SetValue(fiter, 5, dstr)             // nolint: errcheck
	w.store.Remove(iter)
} // func (w *RWin) fileSetProgram(f *objects.File, p *objects.Program)

func (w *RWin) editFile(f *objects.File, iter *gtk.TreeIter) {
	w.displayMsg("IMPLEMENTME!")
} // func (w *RWin) editFile(f *objects.File, iter *gtk.TreeIter)

func (w *RWin) editProgram(p *objects.Program, iter *gtk.TreeIter) {
	var (
		err                    error
		dlg                    *gtk.Dialog
		dbox                   *gtk.Box
		grid                   *gtk.Grid
		titleE, creatorE, urlE *gtk.Entry
		titleL, creatorL, urlL *gtk.Label
	)

	if dlg, err = gtk.DialogNewWithButtons(
		"Edit Program",
		w.win,
		gtk.DIALOG_MODAL,
		[]any{
			"_Cancel",
			gtk.RESPONSE_CANCEL,
			"_OK",
			gtk.RESPONSE_OK,
		},
	); err != nil {
		w.log.Printf("[ERROR] Cannot create dialog for editing  Program %s: %s\n",
			p.Title,
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
	} else if urlL, err = gtk.LabelNew("URL:"); err != nil {
		w.log.Printf("[ERROR] Cannot create URL Label: %s\n",
			err.Error())
		return
	} else if titleL, err = gtk.LabelNew("Title:"); err != nil {
		w.log.Printf("[ERROR] Cannot create Title Label: %s\n",
			err.Error())
		return
	} else if creatorL, err = gtk.LabelNew("Creator:"); err != nil {
		w.log.Printf("[ERROR] Cannot create Creator Label: %s\n",
			err.Error())
		return
	} else if urlE, err = gtk.EntryNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create URL Entry: %s\n",
			err.Error())
		return
	} else if titleE, err = gtk.EntryNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Title Entry: %s\n",
			err.Error())
		return
	} else if creatorE, err = gtk.EntryNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Creator Entry: %s\n",
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

	grid.Attach(titleL, 0, 0, 1, 1)
	grid.Attach(urlL, 0, 1, 1, 1)
	grid.Attach(creatorL, 0, 2, 1, 1)
	grid.Attach(titleE, 1, 0, 1, 1)
	grid.Attach(urlE, 1, 1, 1, 1)
	grid.Attach(creatorE, 1, 2, 1, 1)

	titleE.SetText(p.Title)
	urlE.SetText(p.URLString())
	creatorE.SetText(p.Creator)

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

	var (
		ttl, uri, creator string
		c                 *db.Database
	)

	ttl, _ = titleE.GetText()
	uri, _ = urlE.GetText()
	creator, _ = creatorE.GetText()

	c = w.pool.Get()
	defer w.pool.Put(c)

	var txStatus = true

	if err = c.Begin(); err != nil {
		w.log.Printf("[ERROR] Cannot initiate transaction: %s\n",
			err.Error())
		return
	}

	if ttl != p.Title {
		w.log.Printf("[DEBUG] Title: %q -> %q\n",
			p.Title,
			ttl)
		if err = c.ProgramSetTitle(p, ttl); err != nil {
			w.log.Printf("[ERROR] Cannot update title: %s\n",
				err.Error())
			txStatus = false
			goto FINISH
		} else {
			w.store.SetValue(iter, 1, ttl) // nolint: errcheck
		}
	}

	if uri != p.URLString() {
		w.log.Printf("[DEBUG] URL: %q -> %q\n",
			p.URLString(),
			uri)
		var u *url.URL

		if u, err = url.Parse(uri); err != nil {
			w.log.Printf("[ERROR] Invalid URL %q: %s\n",
				uri,
				err.Error())
		} else if err = c.ProgramSetURL(p, u); err != nil {
			w.log.Printf("[ERROR] Cannot update URL for Program %q: %s\n",
				p.Title,
				err.Error())
			txStatus = false
			goto FINISH
		}
	}

	if creator != p.Creator {
		w.log.Printf("[DEBUG] Creator: %q -> %q\n",
			p.Creator,
			creator)
		if err = c.ProgramSetCreator(p, creator); err != nil {
			w.log.Printf("[ERROR] Cannot update Creator for Program %q: %s\n",
				p.Title,
				err.Error())
			txStatus = false
			goto FINISH
		}
	}

FINISH:
	if txStatus {
		if err = c.Commit(); err != nil {
			w.log.Printf("[ERROR] Cannot commit transaction: %s\n",
				err.Error())
		}
	} else if err = c.Rollback(); err != nil {
		w.log.Printf("[ERROR] Cannot rollback transaction: %s\n",
			err.Error())
	}
} // func (w *RWin) editProgram(p *objects.Program)

func (w *RWin) playProgram(p *objects.Program) {
	var err error

	if err = w.playerCreate(); err != nil {
		w.log.Printf("[ERROR] Cannot start player: %s\n",
			err.Error())
	}

	time.Sleep(time.Millisecond * 2000)

	if err = w.playerClearPlaylist(); err != nil {
		w.log.Printf("[ERROR] Cannot clear playlist: %s\n",
			err.Error())
	} else {
		w.playerPlayProgram(p)
	}
} // func (w *RWin) playProgram(p *objects.Program)

func (w *RWin) cmpIter(m *gtk.TreeModel, a, b *gtk.TreeIter) int {
	var (
		err                    error
		val                    *glib.Value
		gv                     any
		pid1, fid1, pid2, fid2 int64
	)

	if val, err = m.GetValue(a, 0); err != nil {
		w.log.Printf("[ERROR] Cannot get value from column 0: %s\n",
			err.Error())
		return 0
	} else if gv, err = val.GoValue(); err != nil {
		w.log.Printf("[ERROR] Cannot convert GLib value to Go value: %s\n",
			err.Error())
		return 0
	}

	switch v := gv.(type) {
	case int:
		pid1 = int64(v)
	case int64:
		pid1 = v
	default:
		w.log.Printf("[CANTHAPPEN] Unexpected type for PID Column: %T (%#v)\n",
			v,
			v)
		return 0
	}

	if val, err = m.GetValue(a, 2); err != nil {
		w.log.Printf("[ERROR] Cannot get value from column 2: %s\n",
			err.Error())
		return 0
	} else if gv, err = val.GoValue(); err != nil {
		w.log.Printf("[ERROR] Cannot convert GLib value to Go value: %s\n",
			err.Error())
		return 0
	}

	switch v := gv.(type) {
	case int:
		fid1 = int64(v)
	case int64:
		fid1 = v
	default:
		w.log.Printf("[CANTHAPPEN] Unexpected type for FID Column: %T (%#v)\n",
			v,
			v)
		return 0
	}

	if val, err = m.GetValue(b, 0); err != nil {
		w.log.Printf("[ERROR] Cannot get value from column 0: %s\n",
			err.Error())
		return 0
	} else if gv, err = val.GoValue(); err != nil {
		w.log.Printf("[ERROR] Cannot convert GLib value to Go value: %s\n",
			err.Error())
		return 0
	}

	switch v := gv.(type) {
	case int:
		pid2 = int64(v)
	case int64:
		pid2 = v
	default:
		w.log.Printf("[CANTHAPPEN] Unexpected type for PID Column: %T (%#v)\n",
			v,
			v)
		return 0
	}

	if val, err = m.GetValue(b, 2); err != nil {
		w.log.Printf("[ERROR] Cannot get value from column 2: %s\n",
			err.Error())
		return 0
	} else if gv, err = val.GoValue(); err != nil {
		w.log.Printf("[ERROR] Cannot convert GLib value to Go value: %s\n",
			err.Error())
		return 0
	}

	switch v := gv.(type) {
	case int:
		fid2 = int64(v)
	case int64:
		fid2 = v
	default:
		w.log.Printf("[CANTHAPPEN] Unexpected type for FID Column: %T (%#v)\n",
			v,
			v)
		return 0
	}

	var (
		prog1, prog2 *objects.Program
		file1, file2 *objects.File
	)

	prog1 = w.pCache[abs(pid1)]
	prog2 = w.pCache[abs(pid2)]
	file1 = w.fCache[fid1]
	file2 = w.fCache[fid2]

	if pid1 == pid2 {
		if pid1 >= 0 {
			return 0
		}

		return strings.Compare(file1.DisplayTitle(), file2.DisplayTitle())
	} else if pid1 == 0 {
		return -1
	} else if pid2 == 0 {
		return 1
	} else if pid1 > 0 && pid2 > 0 {
		return strings.Compare(prog1.Title, prog2.Title)
	} else {
		return strings.Compare(prog1.Title, prog2.Title)
	}
} // func (w *RWin) cmpIter(m *gtk.TreeModel, a, b *gtk.TreeIter) int

func (w *RWin) getPidFid(iter *gtk.TreeIter) (int64, int64, error) {
	var (
		err      error
		pid, fid int64
		val      *glib.Value
		gv       any
	)

	if val, err = w.store.GetValue(iter, 0); err != nil {
		w.log.Printf("[ERROR] Cannot get value from TreeIter: %s\n",
			err.Error())
		return 0, 0, err
	} else if gv, err = val.GoValue(); err != nil {
		w.log.Printf("[ERROR] Cannot get Go value from GLib value: %s\n",
			err.Error())
		return 0, 0, err
	}

	switch v := gv.(type) {
	case int:
		pid = int64(v)
	case int64:
		pid = v
	default:
		w.log.Printf("[CANTHAPPEN] Unexpected type for ID column: %T\n",
			v)
		return 0, 0, fmt.Errorf("Unexpected type for ID column: %T",
			v)
	}

	if val, err = w.store.GetValue(iter, 2); err != nil {
		w.log.Printf("[ERROR] Cannot get value for column 2: %s\n",
			err.Error())
		return 0, 0, err
	} else if gv, err = val.GoValue(); err != nil {
		w.log.Printf("[ERROR] Cannot get value from GLib value: %s\n",
			err.Error())
		return 0, 0, err
	}

	switch v := gv.(type) {
	case int:
		fid = int64(v)
	case int64:
		fid = v
	default:
		w.log.Printf("[CANTHAPPEN] Unexpected type for ID column: %T\n",
			v)
		return 0, 0, err
	}

	return pid, fid, nil
} // func (w *RWin) getPidFid(iter *gtk.TreeIter) (int64, int64, error)

func (w *RWin) refreshFolders() bool {
	var (
		err     error
		c       *db.Database
		folders []objects.Folder
	)

	w.log.Printf("[TRACE] Checking if folders need to be re-scanned.\n")

	c = w.pool.Get()
	defer w.pool.Put(c)

	if folders, err = c.FolderGetAll(); err != nil {
		w.log.Printf("[ERROR] Cannot load list of Folders: %s\n",
			err.Error())
		return true
	}

	for _, f := range folders {
		var d = f.SinceLastScan()
		if d > rescanInterval {
			w.log.Printf("[DEBUG] Re-scanning folder %s\n",
				f.Path)
			go w.scanner.Walk(f.Path) // nolint: errcheck
		}
	}

	return true
} // func (w *RWin) refreshFolders() bool

func abs(n int64) int64 {
	if n == math.MinInt32 {
		return 0
	} else if n < 0 {
		return -n
	} else {
		return n
	}
} // func abs(n int64) int64

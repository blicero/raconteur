// /home/krylon/go/src/github.com/blicero/raconteur/ui/init.go
// -*- mode: go; coding: utf-8; -*-
// Created on 16. 09. 2023 by Benjamin Walkenhorst
// (c) 2023 Benjamin Walkenhorst
// Time-stamp: <2023-09-20 16:48:10 krylon>

package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"time"

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
	} else if win.playBox, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 1); err != nil {
		win.log.Printf("[ERROR] Cannot create play Box: %s\n",
			err.Error())
		return nil, err
	} else if win.playB, err = gtk.ButtonNew(); err != nil {
		win.log.Printf("[ERROR] Cannot create play Button: %s\n",
			err.Error())
		return nil, err
	} else if win.stopB, err = gtk.ButtonNew(); err != nil {
		win.log.Printf("[ERROR] Cannot create stop Button: %s\n",
			err.Error())
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

	// Set the icons for the buttons
	if win.pauseIcon, err = readIcon("media-playback-pause.png"); err != nil {
		win.log.Printf("[ERROR] Cannot load icon Pause: %s\n",
			err.Error())
		return nil, err
	} else if win.playIcon, err = readIcon("media-playback-start.png"); err != nil {
		win.log.Printf("[ERROR] Cannot load icon Play: %s\n",
			err.Error())
		return nil, err
	} else if win.stopIcon, err = readIcon("media-playback-stop.png"); err != nil {
		win.log.Printf("[ERROR] Cannot load icon Stop: %s\n",
			err.Error())
		return nil, err
	}

	win.playB.SetImage(win.playIcon)
	win.stopB.SetImage(win.stopIcon)

	win.playBox.Add(win.playB)
	win.playBox.Add(win.stopB)
	win.searchBox.Add(win.searchLbl)
	win.searchBox.Add(win.searchEntry)
	win.win.Add(win.mainBox)
	win.scr.Add(win.view)
	win.mainBox.PackStart(win.menu, false, false, 1)
	win.mainBox.PackStart(win.playBox, false, false, 1)
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
			// s  string
		)

		for {
			time.Sleep(time.Second * 5)
			if _, ex = win.getPlayerStatus(); ex != nil {
				win.log.Printf("[ERROR] Cannot query player status: %s\n",
					ex.Error())
			} /*else {
				win.log.Printf("[DEBUG] Player is %s\n", s)
			}*/

		}
	}()

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

func readIcon(name string) (*gtk.Image, error) {
	var (
		err         error
		path        string
		content     []byte
		icon, small *gdk.Pixbuf
		img         *gtk.Image
	)

	path = filepath.Join("icons", name)

	if content, err = icons.ReadFile(path); err != nil {
		return nil, err
	} else if icon, err = gdk.PixbufNewFromDataOnly(content); err != nil {
		return nil, err
	} else if small, err = icon.ScaleSimple(64, 64, gdk.INTERP_HYPER); err != nil {
		return nil, err
	} else if img, err = gtk.ImageNewFromPixbuf(small); err != nil {
		return nil, err
	}

	return img, nil
} // func readIcon(name string) (*gtk.Image, error)

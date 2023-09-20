// /home/krylon/go/src/github.com/blicero/raconteur/ui/junk.go
// -*- mode: go; coding: utf-8; -*-
// Created on 16. 09. 2023 by Benjamin Walkenhorst
// (c) 2023 Benjamin Walkenhorst
// Time-stamp: <2023-09-16 21:40:20 krylon>

package ui

// This file is my trash bin / museum of code I am not (currently) using but
// want to keep around for possible future reference.

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

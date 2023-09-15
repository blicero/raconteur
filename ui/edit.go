// /home/krylon/go/src/github.com/blicero/raconteur/ui/edit.go
// -*- mode: go; coding: utf-8; -*-
// Created on 15. 09. 2023 by Benjamin Walkenhorst
// (c) 2023 Benjamin Walkenhorst
// Time-stamp: <2023-09-15 19:25:14 krylon>

package ui

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/objects"
	"github.com/gotk3/gotk3/gtk"
)

func (w *RWin) editFile(f *objects.File, iter *gtk.TreeIter) {
	var (
		err                         error
		dlg                         *gtk.Dialog
		dbox                        *gtk.Box
		grid                        *gtk.Grid
		titleE, urlE                *gtk.Entry
		trackE, discE               *gtk.SpinButton
		titleL, urlL, trackL, discL *gtk.Label
	)

	if dlg, err = gtk.DialogNewWithButtons(
		"Edit Title",
		w.win,
		gtk.DIALOG_MODAL,
		[]any{
			"_Cancel",
			gtk.RESPONSE_CANCEL,
			"_OK",
			gtk.RESPONSE_OK,
		},
	); err != nil {
		w.log.Printf("[ERROR] Cannot create dialog for editing File %s: %s\n",
			f.Title,
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
	} else if titleL, err = gtk.LabelNew("Title:"); err != nil {
		w.log.Printf("[ERROR] Cannot create Title Label: %s\n",
			err.Error())
		return
	} else if urlL, err = gtk.LabelNew("URL:"); err != nil {
		w.log.Printf("[ERROR] Cannot create URL Label: %s\n",
			err.Error())
		return
	} else if trackL, err = gtk.LabelNew("Track:"); err != nil {
		w.log.Printf("[ERROR] Cannot create Track Label: %s\n",
			err.Error())
		return
	} else if discL, err = gtk.LabelNew("Disc:"); err != nil {
		w.log.Printf("[ERROR] Cannot create Disc Label: %s\n",
			err.Error())
		return
	} else if titleE, err = gtk.EntryNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create Title Entry: %s\n",
			err.Error())
		return
	} else if urlE, err = gtk.EntryNew(); err != nil {
		w.log.Printf("[ERROR] Cannot create URL Entry: %s\n",
			err.Error())
		return
	} else if trackE, err = gtk.SpinButtonNewWithRange(1, 2000, 1); err != nil {
		w.log.Printf("[ERROR] Cannot create Track SpinButton: %s\n",
			err.Error())
		return
	} else if discE, err = gtk.SpinButtonNewWithRange(1, 100, 1); err != nil {
		w.log.Printf("[ERROR] Cannot create Disc SpinButton: %s\n",
			err.Error())
		return
	} else if dbox, err = dlg.GetContentArea(); err != nil {
		w.log.Printf("[ERROR] Cannot get ContentArea of EditFile Dialog: %s\n",
			err.Error())
		return
	}

	grid.InsertColumn(0)
	grid.InsertColumn(1)

	grid.InsertRow(0)
	grid.InsertRow(1)
	grid.InsertRow(2)
	grid.InsertRow(3)

	grid.Attach(titleL, 0, 0, 1, 1)
	grid.Attach(titleE, 1, 0, 1, 1)
	grid.Attach(urlL, 0, 1, 1, 1)
	grid.Attach(urlE, 1, 1, 1, 1)
	grid.Attach(discL, 0, 2, 1, 1)
	grid.Attach(discE, 1, 2, 1, 1)
	grid.Attach(trackL, 0, 3, 1, 1)
	grid.Attach(trackE, 1, 3, 1, 1)

	titleE.SetText(f.DisplayTitle())
	urlE.SetText(f.URL)
	discE.SetValue(float64(f.Ord[0]))
	trackE.SetValue(float64(f.Ord[1]))

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
		title, uri  string
		track, disc int
		txstatus    bool
		d           *db.Database
	)

	title, _ = titleE.GetText()
	uri, _ = urlE.GetText()
	disc = discE.GetValueAsInt()
	track = trackE.GetValueAsInt()

	d = w.pool.Get()
	defer w.pool.Put(d)

	if err = d.Begin(); err != nil {
		w.log.Printf("[ERROR] Cannot start transaction: %s\n",
			err.Error())
		return
	}

	defer func() {
		if txstatus {
			d.Commit() // nolint: errcheck
		} else {
			d.Rollback() // nolint: errcheck
		}
	}()

	if title != f.Title {
		w.log.Printf("[DEBUG] Title: %q -> %q\n",
			f.Title, title)
		if err = d.FileSetTitle(f, title); err != nil {
			w.log.Printf("[ERROR] Cannot set Title of File %d (%s) to %s: %s\n",
				f.ID,
				f.Title,
				title,
				err.Error())
			return
		}
	}

	if uri != f.URL {
		w.log.Printf("[DEBUG] URL: %q -> %q\n",
			f.URL, uri)
		w.displayMsg("Updating File URL is not implemented yet")
	}

	if disc != int(f.Ord[0]) || track != int(f.Ord[1]) {
		w.log.Printf("[DEBUG] Ord: %d/%d -> %d/%d\n",
			f.Ord[0], f.Ord[1],
			disc, track)
		var ord = []int64{int64(disc), int64(track)}
		if err = d.FileSetOrd(f, ord); err != nil {
			w.log.Printf("[ERROR] Cannot set Disc/Track of File %d (%s): %s\n",
				f.ID,
				f.Title,
				err.Error())
			return
		}
	}

	txstatus = true
} // func (w *RWin) editFile(f *objects.File, iter *gtk.TreeIter)

func (w *RWin) editProgram(p *objects.Program, iter *gtk.TreeIter) {
	var (
		err                           error
		msg                           string
		c                             *db.Database
		dlg                           *gtk.Dialog
		dbox                          *gtk.Box
		grid                          *gtk.Grid
		titleE, creatorE, urlE        *gtk.Entry
		titleL, creatorL, urlL, fileL *gtk.Label
		curfileBox                    *gtk.ComboBoxText
		files                         []objects.File
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
		w.log.Printf("[ERROR] Cannot create dialog for editing Program %s: %s\n",
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
	} else if fileL, err = gtk.LabelNew("Current file:"); err != nil {
		w.log.Printf("[ERROR] Cannot create File Label: %s\n",
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
		w.log.Printf("[ERROR] Cannot get ContentArea of EditProgram Dialog: %s\n",
			err.Error())
		return
	}

	c = w.pool.Get()
	defer w.pool.Put(c)

	if files, err = c.FileGetByProgram(p); err != nil {
		msg = fmt.Sprintf("Failed to load files for Program %d (%q): %s",
			p.ID,
			p.Title,
			err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
		return
	} else if curfileBox, err = gtk.ComboBoxTextNew(); err != nil {
		msg = fmt.Sprintf("Cannot create ComboBox for Files: %s", err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
		return
	}

	var (
		fileID    = make(map[string]int64, len(files))
		fileNames = make([]string, len(files))
		activeIdx int
	)

	for idx, f := range files {
		fileID[f.Title] = f.ID
		fileNames[idx] = f.Title
		curfileBox.Append(strconv.FormatInt(f.ID, 10), f.Title)
		if p.CurFile == f.ID {
			activeIdx = idx
		}
	}

	curfileBox.SetActive(activeIdx)

	grid.InsertColumn(0)
	grid.InsertColumn(1)
	grid.InsertRow(0)
	grid.InsertRow(1)
	grid.InsertRow(2)
	grid.InsertRow(3)

	grid.Attach(titleL, 0, 0, 1, 1)
	grid.Attach(urlL, 0, 1, 1, 1)
	grid.Attach(creatorL, 0, 2, 1, 1)
	grid.Attach(fileL, 0, 3, 1, 1)
	grid.Attach(titleE, 1, 0, 1, 1)
	grid.Attach(urlE, 1, 1, 1, 1)
	grid.Attach(creatorE, 1, 2, 1, 1)
	grid.Attach(curfileBox, 1, 3, 1, 1)

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

	var ttl, uri, creator string

	ttl, _ = titleE.GetText()
	uri, _ = urlE.GetText()
	creator, _ = creatorE.GetText()

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

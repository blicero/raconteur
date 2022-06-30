// /home/krylon/go/src/github.com/blicero/raconteur/ui/play.go
// -*- mode: go; coding: utf-8; -*-
// Created on 15. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-30 19:47:20 krylon>

// +build ignore

package ui

import (
	"fmt"
	"net/url"
	"os/exec"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/objects"
	"github.com/davecgh/go-spew/spew"
	"github.com/godbus/dbus/v5"
	"github.com/gotk3/gotk3/gtk"
)

// nolint: deadcode,unused,varcheck
const (
	playerPath     = "/usr/bin/vlc"
	objName        = "org.mpris.MediaPlayer2.vlc"
	objPath        = "/org/mpris/MediaPlayer2"
	objAddMatch    = "org.freedesktop.DBus.AddMatch"
	objInterface   = "org.mpris.MediaPlayer2.Player"
	trackInterface = "org.mpris.MediaPlayer2.TrackList"
	trackList      = "org.mpris.MediaPlayer2.TrackList.Tracks"
	noTrack        = "/org/mpris/MediaPlayer2/TrackList/NoTrack"
	addTrack       = "org.mpris.MediaPlayer2.TrackList.AddTrack"
	delTrack       = "org.mpris.MediaPlayer2.TrackList.RemoveTrack"
	playerPlay     = "org.mpris.MediaPlayer2.Player.Play"
	propStatus     = "org.mpris.MediaPlayer2.Player.PlaybackStatus"
	propPosition   = "org.mpris.MediaPlayer2.Player.Position"
	propMeta       = "org.mpris.MediaPlayer2.Player.Metadata"
	propTracklist  = "org.mpris.MediaPlayer2.TrackList.Tracks"
	signalSeeked   = "/org/mpris/MediaPlayer2/Player/Seeked"
	signalTrackAdd = "/org/mpris/MediaPlayer2/Player/TrackAdded"
)

func (w *RWin) getPlayerStatus() (string, error) {
	var (
		err error
		str string
		val dbus.Variant
		obj = w.mbus.Object(objName, objPath)
	)

	w.log.Printf("[TRACE] getPlayerStatus - ENTER\n")

	if val, err = obj.GetProperty(propStatus); err != nil {
		// w.log.Printf("[ERROR] Cannot get player status: %s\n",
		// 	err.Error())
		return "", err
	}

	str = val.Value().(string)

	w.log.Printf("[DEBUG] PlaybackStatus is %s\n",
		str)

	if !(str == "Playing" || str == "Paused") {
		return str, nil
	}
	var (
		meta map[string]dbus.Variant
		pos  int64
		ok   bool
	)

	if val, err = obj.GetProperty(propPosition); err != nil {
		w.log.Printf("[ERROR] Cannot get Position: %s\n",
			err.Error())
		return "", err
	} else if pos, ok = val.Value().(int64); !ok {
		w.log.Printf("[ERROR] Cannot convert result to int64: %T\n",
			val.Value())
		return "", fmt.Errorf("Cannot convert result to int64: %T",
			val.Value())
	} else if val, err = obj.GetProperty(propMeta); err != nil {
		w.log.Printf("[ERROR] Cannot get Property %s: %s\n",
			propMeta,
			err.Error())
		return "", err
	} else if meta, ok = val.Value().(map[string]dbus.Variant); !ok {
		w.log.Printf("[ERROR] Wrong type for %s: %T\n",
			propMeta,
			val.Value())
		return "", fmt.Errorf("Wrong type for %s: %T",
			propMeta,
			val.Value())
	}

	var sec = time.Microsecond * time.Duration(pos)

	w.log.Printf("[DEBUG] Player is at position %s\n",
		sec)

	// if common.Debug {
	// 	for k, v := range meta {
	// 		w.log.Printf("[DEBUG] Meta %-15s => (%T) %#v\n",
	// 			k,
	// 			v.Value(),
	// 			v.Value())
	// 	}
	// }

	var (
		uriRaw, uriEsc string
		fileURL        *url.URL
	)

	uriRaw = meta["xesam:url"].Value().(string)

	if uriEsc, err = url.PathUnescape(uriRaw); err != nil {
		w.log.Printf("[ERROR] Cannot un-escape URL path %q: %s\n",
			uriRaw,
			err.Error())
		return "", err
	} else if fileURL, err = url.Parse(uriEsc); err != nil {
		w.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
			uriEsc,
			err.Error())
		return "", err
	} else if common.Debug {
		w.log.Printf("[DEBUG] Currently playing %s at %s\n",
			fileURL.Path,
			sec)
	}

	var (
		c        *db.Database
		f        *objects.File
		p        *objects.Program
		txStatus bool
	)

	c = w.pool.Get()
	defer w.pool.Put(c)

	if err = c.Begin(); err != nil {
		w.log.Printf("[ERROR] Cannot start transaction: %s\n",
			err.Error())
		return "", err
	}

	defer func() {
		if txStatus {
			w.log.Printf("[TRACE] COMMIT Transaction\n")
			c.Commit() // nolint: errcheck
		} else {
			w.log.Printf("[TRACE] ROLLBACK Transaction\n")
			c.Rollback() // nolint: errcheck
		}
	}()

	if f, err = c.FileGetByPath(fileURL.Path); err != nil {
		w.log.Printf("[ERROR] Cannot look for File %s: %s\n",
			fileURL.Path,
			err.Error())
		return "", err
	} else if err = c.FileSetPosition(f, pos); err != nil {
		w.log.Printf("[ERROR] Cannot set Position for File %q to %s: %s\n",
			f.DisplayTitle(),
			sec,
			err.Error())
		return "", err
	} else if f.ProgramID == 0 {
		w.log.Printf("[DEBUG] File %q does not belong to any Program.\n",
			f.DisplayTitle())
		return str, nil
	} else if p, err = c.ProgramGetByID(f.ProgramID); err != nil {
		w.log.Printf("[ERROR] Cannot lookup Program %d: %s\n",
			f.ProgramID,
			err.Error())
		return "", err
	} else if p == nil {
		w.log.Printf("[CANTHAPPEN] Program %d was not found in database.\n",
			f.ProgramID)
		return "",
			fmt.Errorf("Program %d was not found in database",
				f.ProgramID)
	} else if p.CurFile == f.ID {
		// return str, nil
	} else if err = c.ProgramSetCurFile(p, f); err != nil {
		w.log.Printf("[ERROR] Cannot set current file for Program %q (%d) to %d (%q): %s\n",
			p.Title,
			p.ID,
			f.ID,
			f.DisplayTitle(),
			err.Error())
		return "", err
	}

	w.log.Printf("[DEBUG] Set txStatus = true\n")
	txStatus = true

	return str, nil
} // func (w *RWin) getPlayerStatus() (string, error)

func (w *RWin) playerCreate() error {
	krylib.Trace()
	defer fmt.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	w.lock.Lock()
	defer w.lock.Unlock()

	if w.playerActive {
		w.log.Printf("[INFO] Player is already active?\n")
		return nil
	}

	var cmd = exec.Command(
		playerPath,
		"--no-fullscreen",
		// "-no-close-at-end",
	)

	if err := cmd.Start(); err != nil {
		w.log.Printf("[ERROR] Cannot start player %s: %s\n",
			playerPath,
			err.Error())
		return err
	}

	w.playerActive = true
	go w.playerTimeout(cmd)
	w.playerRegisterSignals()

	return nil
} // func (w *RWin) playerCreate() error

func (w *RWin) playerTimeout(proc *exec.Cmd) {
	var err error

	time.Sleep(time.Second * 2)

	if err = proc.Wait(); err != nil {
		w.log.Printf("[ERROR] Player exited with error: %s\n",
			err.Error())
	}

	w.lock.Lock()
	w.playerActive = false
	w.lock.Unlock()
} // func (w *RWin) playerTimeout()

func (w *RWin) playerRegisterSignals() error {
	krylib.Trace()

	w.mbus.Signal(w.sigq)

	var obj = w.mbus.BusObject()
	var res = obj.AddMatchSignal(
		trackInterface,
		"TrackAdded",
		dbus.WithMatchObjectPath(objPath),
		dbus.WithMatchOption("sender", objName),
	)

	if res.Err != nil {
		w.log.Printf("[ERROR] Cannot register Signal TrackAdd: %s\n",
			res.Err.Error())
		return res.Err
	}

	w.log.Printf("[DEBUG] Result of registering Signal: %s\n",
		spew.Sdump(res))

	go w.playerProcessSignals()

	// time.Sleep(time.Millisecond * 1000)

	return nil
} // func (w *RWin) playerRegisterSignals() error

func (w *RWin) playerProcessSignals() {
	krylib.Trace()
	for v := range w.sigq {
		w.log.Printf("[INFO] %T => %#v\n\n%s\n",
			v,
			v,
			spew.Sdump(v))
	}

	w.log.Printf("[DEBUG] playerProcessSignals is quitting.\n")
} // func (w *RWin) playerProcessSignals()

func (w *RWin) playerPlayProgram(p *objects.Program) {
	krylib.Trace()
	defer fmt.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	var (
		err   error
		c     *db.Database
		files []objects.File
		obj   = w.mbus.Object(objName, objPath)
	)

	c = w.pool.Get()
	defer w.pool.Put(c)

	if files, err = c.FileGetByProgram(p); err != nil {
		var msg = fmt.Sprintf("Cannot get Files for Program %q (%d): %s",
			p.Title,
			p.ID,
			err.Error())
		w.log.Println("[ERROR] " + msg)
		w.displayMsg(msg)
		return
	}

	// for _, f := range files {
	for i := len(files) - 1; i >= 0; i-- {
		f := files[i]

		w.log.Printf("[TRACE] Add %s to Playlist\n",
			f.DisplayTitle())

		var (
			val   dbus.Variant
			track = dbus.ObjectPath(noTrack)
		)

		if val, err = obj.GetProperty(trackList); err != nil {
			w.log.Printf("[ERROR] Cannot get TrackList %s: %s\n",
				propTracklist,
				err.Error())
			track = dbus.ObjectPath(noTrack)
		} else {
			var list = val.Value().([]dbus.ObjectPath)

			w.log.Printf("[DEBUG] %s = %T => %s\n%s\n",
				trackList,
				val,
				spew.Sdump(val),
				spew.Sdump(list),
			)

			if len(list) == 0 {
				track = dbus.ObjectPath(noTrack)
			} else {
				track = list[len(list)-1]
			}
		}

		var res = obj.Call(
			addTrack,
			0, // dbus.FlagNoReplyExpected,
			f.PathURL(),
			track, //dbus.ObjectPath(noTrack),
			false,
		)

		gtk.MainIterationDo(false)

		if res.Err != nil {
			w.log.Printf("[ERROR] DBus method call failed: %s\n",
				res.Err.Error())
		} else {
			if common.Debug {
				w.log.Printf("[DEBUG] AddTrack returned %s\n",
					spew.Sdump(res))
			}
			time.Sleep(time.Millisecond * 100)
		}
	}

	// time.Sleep(time.Second)

	for i := 0; i < 3; i++ {
		gtk.MainIterationDo(false)
	}

	obj.Call(
		playerPlay,
		dbus.FlagNoReplyExpected,
	)

	// TODO Jump to the current file and position!
} // func (w *RWin) playerPlayProgram(p *objects.Program)

func (w *RWin) playerClearPlaylist() error {
	krylib.Trace()
	defer fmt.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	var (
		err error
		val dbus.Variant
		obj = w.mbus.Object(objName, objPath)
	)

	if val, err = obj.GetProperty(trackList); err != nil {
		w.log.Printf("[ERROR] Cannot get TrackList from Player: %s\n",
			err.Error())
		return err
	}

	var items = val.Value().([]dbus.ObjectPath)

	if common.Debug {
		w.log.Printf("[DEBUG] Current Playlist: %s\n",
			spew.Sdump(items))
	}

	for _, i := range items {
		w.log.Printf("[TRACE] Process %s\n",
			i)
		obj.Call(
			delTrack,
			dbus.FlagNoReplyExpected,
			i)
	}

	return nil
} // func (w *RWin) playerClearPlaylist() error

// I thought I could listen for Signals from my player to notice when the
// track changes or something like that, BUT it turns out VLC has no
// useful Signals to deliver at all.
// But subscribing to signals is not that trivial, hence I leave this
// commented-out method here for future reference.

// func (w *RWin) registerSignal() {
// 	w.log.Printf("[TRACE] Subscribing to signals on DBus\n")
// 	w.mbus.BusObject().Call(
// 		objName,
// 		0,
// 		"type='signal',path='/org/mpris/MediaPlayer2/Player/Seeked',interface='org.mpris.MediaPlayer2.Player',sender='org.mpris.MediaPlayer2.smplayer'")

// 	var ch = make(chan *dbus.Signal, 5)
// 	w.log.Printf("[TRACE] Asking for signals\n")
// 	w.mbus.Signal(ch)

// 	go func() {
// 		w.log.Printf("[TRACE] Receiving from queue\n")
// 		for v := range ch {
// 			w.log.Printf("[DEBUG] Got %T from DBus: %s\n",
// 				v,
// 				spew.Sdump(v))
// 		}
// 	}()
// } // func (w *Rwin) registerSignal()

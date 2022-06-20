// /home/krylon/go/src/github.com/blicero/raconteur/ui/play.go
// -*- mode: go; coding: utf-8; -*-
// Created on 15. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-20 22:28:14 krylon>

package ui

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/objects"
	"github.com/godbus/dbus/v5"
)

const (
	playerPath     = "/usr/bin/vlc"
	objName        = "org.mpris.MediaPlayer2.vlc"
	objPath        = "/org/mpris/MediaPlayer2"
	objInterface   = "org.mpris.MediaPlayer2.Player"
	trackInterface = "org.mpris.MediaPlayer2.TrackList"
	trackList      = "org.mpris.MediaPlayer2.TrackList.Tracks"
	noTrack        = "/org/mpris/MediaPlayer2/TrackList/NoTrack"
	addTrack       = "org.mpris.MediaPlayer2.TrackList.AddTrack"
	delTrack       = "org.mpris.MediaPlayer2.TrackList.RemoveTrack"
)

func (w *RWin) getPlayerStatus() (string, error) {
	var (
		err error
		str string
		val dbus.Variant
		obj = w.mbus.Object(objName, objPath)
	)

	if val, err = obj.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus"); err != nil {
		// w.log.Printf("[ERROR] Cannot get player status: %s\n",
		// 	err.Error())
		return "", err
	}

	str = val.Value().(string)

	w.log.Printf("[DEBUG] PlaybackStatus is %s\n",
		str)

	if str == "Playing" {
		// get the file and position, save it
	}

	return str, nil
} // func (w *RWin) getPlayerStatus() (string, error)

func (w *RWin) playerCreate() error {
	krylib.Trace()
	defer fmt.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	w.lock.Lock()
	defer w.lock.Unlock()

	if !w.playerActive {
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

		return nil
	} else {
		w.log.Printf("[DEBUG] Player %s is already started.\n",
			playerPath)
		return nil
	}
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

	for _, f := range files {
		w.log.Printf("[TRACE] Add %s to Playlist\n",
			f.DisplayTitle())

		var res = obj.Call(
			addTrack,
			0, // dbus.FlagNoReplyExpected,
			f.PathURL(),
			dbus.ObjectPath(noTrack),
			false,
		)

		if res.Err != nil {
			w.log.Printf("[ERROR] DBus method call failed: %s\n",
				res.Err.Error())
		}
	}

	obj.Call(
		"org.mpris.MediaPlayer2.Player.Play",
		dbus.FlagNoReplyExpected,
	)
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

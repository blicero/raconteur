// /home/krylon/go/src/github.com/blicero/raconteur/ui/play.go
// -*- mode: go; coding: utf-8; -*-
// Created on 15. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-23 19:37:34 krylon>

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
	propStatus     = "org.mpris.MediaPlayer2.Player.PlaybackStatus"
	propPosition   = "org.mpris.MediaPlayer2.Player.Position"
	propMeta       = "org.mpris.MediaPlayer2.Player.Metadata"
)

func (w *RWin) getPlayerStatus() (string, error) {
	var (
		err error
		str string
		val dbus.Variant
		obj = w.mbus.Object(objName, objPath)
	)

	if val, err = obj.GetProperty(propStatus); err != nil {
		// w.log.Printf("[ERROR] Cannot get player status: %s\n",
		// 	err.Error())
		return "", err
	}

	str = val.Value().(string)

	w.log.Printf("[DEBUG] PlaybackStatus is %s\n",
		str)

	if str == "Playing" || str == "Paused" {
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

		if common.Debug {
			for k, v := range meta {
				w.log.Printf("[DEBUG] Meta %-15s => (%T) %#v\n",
					k,
					v.Value(),
					v.Value())
			}
		}

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
			c *db.Database
			f *objects.File
		)

		c = w.pool.Get()
		defer w.pool.Put(c)

		if f, err = c.FileGetByPath(fileURL.Path); err != nil {
			w.log.Printf("[ERROR] Cannot look for File %s: %s\n",
				fileURL.Path,
				err.Error())
			return "", err
		}
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

// /home/krylon/go/src/github.com/blicero/raconteur/ui/mbus.go
// -*- mode: go; coding: utf-8; -*-
// Created on 17. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-18 00:12:57 krylon>

// Methods dealing with DBus

package ui

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/godbus/dbus/v5"
)

const (
	objName      = "org.mpris.MediaPlayer2.smplayer"
	objPath      = "/org/mpris/MediaPlayer2"
	objInterface = "org.mpris.MediaPlayer2.Player"
)

func (w *RWin) registerSignal() {

	w.log.Printf("[TRACE] Issuing call to SessionBus\n")
	w.mbus.BusObject().Call(
		objName,
		0,
		"type='signal',path='/org/mpris/MediaPlayer2/Player/Seeked',interface='org.mpris.MediaPlayer2.Player',sender='org.mpris.MediaPlayer2.smplayer'")

	var ch = make(chan *dbus.Signal, 5)
	w.log.Printf("[TRACE] Asking for signals\n")
	w.mbus.Signal(ch)

	go func() {
		w.log.Printf("[TRACE] Receiving from queue\n")
		for v := range ch {
			w.log.Printf("[DEBUG] Got %T from DBus: %s\n",
				v,
				spew.Sdump(v))
		}
	}()
} // func (w *Rwin) registerSignal()

func (w *RWin) getPlayerStatus() (string, error) {
	var (
		err  error
		str  string
		val  dbus.Variant
		call *dbus.Call
		obj  = w.mbus.Object(objName, objPath)
	)

	if val, err = obj.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus"); err != nil {
		w.log.Printf("[ERROR] Cannot get player status: %s\n",
			err.Error())
		return "", err
	}

	call = obj.AddMatchSignal(objInterface, "Seeked")

	w.log.Printf("[DEBUG] Got dbus.Call: %s\n",
		spew.Sdump(call))

	str = val.Value().(string)

	if str == "Playing" {
		// get the file and position, save it

	}

	return str, nil
} // func (w *RWin) getPlayerStatus() (string, error)

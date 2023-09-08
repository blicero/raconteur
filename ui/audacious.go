// /home/krylon/go/src/github.com/blicero/raconteur/ui/audacious.go
// -*- mode: go; coding: utf-8; -*-
// Created on 08. 09. 2023 by Benjamin Walkenhorst
// (c) 2023 Benjamin Walkenhorst
// Time-stamp: <2023-09-08 18:14:36 krylon>

//go:build ignore

package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/blicero/krylib"
	"github.com/godbus/dbus/v5"
)

// We try to use audacious as our player. It offers a much richer DBus API than
// MPris2 does, so I'll try to use that to its fullest.

const (
	playerCommand = "/usr/bin/audacious"
	objName       = "org.mpris.MediaPlayer2.audacious"
	objPath       = "org/atheme/audacious"
	objInterface  = "org.atheme.audacious"
	methStatus    = "Status"
)

func objMethod(name string) string {
	return objInterface + "." + name
}

func (w *RWin) getPlayerStatus() (string, error) {
	var (
		err      error
		str, msg string
		val      dbus.Variant
		obj      = w.mbus.Object(objName, objPath)
		call     *dbus.Call
	)

	if call = obj.Call(objMethod(methStatus), 0); call == nil {
		msg = fmt.Sprintf("Failed to call method %s on player",
			methStatus)
		w.log.Printf("[ERROR] %s\n", msg)
		return "", errors.New(msg)
	} else if err = call.Store(&str); err != nil {
		msg = fmt.Sprintf("Failed to store return value of method %s: %s",
			methStatus,
			err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		return "", errors.New(msg)
	}

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
		playerCommand,
		"--no-fullscreen",
		// "-no-close-at-end",
	)

	if err := cmd.Start(); err != nil {
		w.log.Printf("[ERROR] Cannot start player %s: %s\n",
			playerCommand,
			err.Error())
		return err
	}

	w.playerActive = true
	// go w.playerTimeout(cmd)
	// w.playerRegisterSignals()

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

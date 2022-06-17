// /home/krylon/go/src/github.com/blicero/raconteur/ui/play.go
// -*- mode: go; coding: utf-8; -*-
// Created on 15. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-17 17:52:28 krylon>

package ui

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/raconteur/db"
	"github.com/blicero/raconteur/objects"
)

const playerPath = "/usr/bin/smplayer"

func (w *RWin) playerCreate() error {
	krylib.Trace()
	defer fmt.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	w.lock.Lock()
	defer w.lock.Unlock()

	if !w.playerActive {
		var cmd = exec.Command(
			playerPath,
			"-no-fullscreen",
			"-no-close-at-end",
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

	var args = make([]string, len(files)+1)

	args[0] = "-add-to-playlist"

	for i, f := range files {
		args[i+1] = f.Path
	}

	var cmd = exec.Command(playerPath, args...)

	if err = cmd.Start(); err != nil {
		var msg = fmt.Sprintf("Failed to run %s: %s",
			playerPath,
			err.Error())
		w.log.Println("[ERROR] " + msg)
		w.displayMsg(msg)
		return
	}
} // func (w *RWin) playerPlayProgram(p *objects.Program)

func (w *RWin) playerClearPlaylist() error {
	krylib.Trace()
	defer fmt.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	var err error
	var proc = exec.Command(
		playerPath,
		"-send-action",
		"pl_remove_all",
	)

	if err = proc.Start(); err != nil {
		w.log.Printf("[ERROR] Cannot run smplayer action pl_remove_all: %s\n",
			err.Error())
		return err
	}

	return nil
} // func (w *RWin) playerClearPlaylist() error

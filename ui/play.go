// /home/krylon/go/src/github.com/blicero/raconteur/ui/play.go
// -*- mode: go; coding: utf-8; -*-
// Created on 15. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2023-09-12 17:45:07 krylon>

package ui

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
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
	playerPath            = "/usr/bin/audacious"
	objName               = "org.atheme.audacious"
	objPath               = "/org/mpris/MediaPlayer2"
	objAddMatch           = "org.freedesktop.DBus.AddMatch"
	objInterface          = "org.mpris.MediaPlayer2.Player"
	audInterface          = "org.atheme.audacious"
	audPath               = "/org/atheme/audacious"
	methStatus            = "Status"
	methPlay              = "Play"
	methSeek              = "Seek"
	methPlaylistCreate    = "NewPlaylist"
	methPlaylistAddFiles  = "AddList"
	methPlaylistOpenFiles = "OpenList"
	methPlaylistRename    = "SetActivePlaylistName"
	methPlaylistPosition  = "Position"
	methPlaylistCnt       = "NumberOfPlaylists"
	methPlaylistSetActive = "SetActivePlaylist"
	methPlaylistGetName   = "GetActivePlaylistName"
	methPlaylistJump      = "Jump"
	methFilename          = "SongFilename"
	methFilePosition      = "Time"
	trackInterface        = "org.mpris.MediaPlayer2.TrackList"
	trackList             = "org.mpris.MediaPlayer2.TrackList.Tracks"
	noTrack               = "/org/mpris/MediaPlayer2/TrackList/NoTrack"
	addTrack              = "org.mpris.MediaPlayer2.TrackList.AddTrack"
	delTrack              = "org.mpris.MediaPlayer2.TrackList.RemoveTrack"
	playerPlay            = "org.mpris.MediaPlayer2.Player.Play"
	propStatus            = "org.mpris.MediaPlayer2.Player.PlaybackStatus"
	propPosition          = "org.mpris.MediaPlayer2.Player.Position"
	propMeta              = "org.mpris.MediaPlayer2.Player.Metadata"
	propTracklist         = "org.mpris.MediaPlayer2.TrackList.Tracks"
	signalSeeked          = "/org/mpris/MediaPlayer2/Player/Seeked"
	signalTrackAdd        = "/org/mpris/MediaPlayer2/Player/TrackAdded"
	dbusFlags             = dbus.FlagAllowInteractiveAuthorization
)

func (w *RWin) getPlayerStatus() (string, error) {
	var (
		err        error
		str, msg   string
		methodName = objMethod(audInterface, methStatus)
		obj        = w.mbus.Object(objName, audPath)
	)

	w.lock.Lock()
	defer w.lock.Unlock()

	// krylib.Trace()

	if err = obj.Call(methodName, dbusFlags).Store(&str); err != nil {
		msg = fmt.Sprintf("Failed to store return value of method %s: %s",
			methStatus,
			err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		return "", errors.New(msg)
	}

	str = strings.ToLower(str)
	w.log.Printf("[TRACE] Player status is %s\n", str)

	if !(str == "playing" || str == "paused") {
		return str, nil
	}

	var (
		ppos     int
		fpos     int64
		filename string
	)

	if ppos, err = w.getPlaylistPosition(); err != nil {
		return "", err
	} else if filename, err = w.getSongFilename(ppos); err != nil {
		return "", err
	} else if fpos, err = w.getSongPosition(); err != nil {
		return "", err
	}

	msg = fmt.Sprintf("Currently playing %s",
		filename)
	w.log.Printf("[DEBUG] %s\n", msg)
	// w.displayMsg(msg)

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

	if f, err = c.FileGetByPath(filename); err != nil {
		msg = fmt.Sprintf("Cannot get file %s from database: %s",
			filename,
			err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
		return "", err
	} else if f == nil {
		w.log.Printf("[DEBUG] File %s was not found in database\n",
			filename)
		return "", nil
	} else if err = c.FileSetPosition(f, fpos); err != nil {
		w.log.Printf("[ERROR] Cannot set Position for File %q to %s: %s\n",
			f.DisplayTitle(),
			fpos,
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

	if w.playerActive.Load() {
		w.log.Printf("[INFO] Player is already active?\n")
		return nil
	}

	var cmd = exec.Command(
		playerPath,
	)

	if err := cmd.Start(); err != nil {
		w.log.Printf("[ERROR] Cannot start player %s: %s\n",
			playerPath,
			err.Error())
		return err
	}

	w.playerActive.Store(true)
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

	w.playerActive.Store(false)
} // func (w *RWin) playerTimeout()

func (w *RWin) playerRegisterSignals() error {
	return nil
	// krylib.Trace()

	// w.mbus.Signal(w.sigq)

	// var obj = w.mbus.BusObject()
	// var res = obj.AddMatchSignal(
	// 	trackInterface,
	// 	"TrackAdded",
	// 	dbus.WithMatchObjectPath(objPath),
	// 	dbus.WithMatchOption("sender", objName),
	// )

	// if res.Err != nil {
	// 	w.log.Printf("[ERROR] Cannot register Signal TrackAdd: %s\n",
	// 		res.Err.Error())
	// 	return res.Err
	// }

	// w.log.Printf("[DEBUG] Result of registering Signal: %s\n",
	// 	spew.Sdump(res))

	// go w.playerProcessSignals()

	// // time.Sleep(time.Millisecond * 1000)

	// return nil
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

	w.lock.Lock()
	defer w.lock.Unlock()

	var (
		err                      error
		msg                      string
		c                        *db.Database
		files                    []objects.File
		playlistNames, fileNames []string
		call                     *dbus.Call
		obj                      = w.mbus.Object(objName, audPath)
	)

	c = w.pool.Get()
	defer w.pool.Put(c)

	if files, err = c.FileGetByProgram(p); err != nil {
		msg = fmt.Sprintf("Cannot get Files for Program %q (%d): %s",
			p.Title,
			p.ID,
			err.Error())
		w.log.Println("[ERROR] " + msg)
		w.displayMsg(msg)
		return
	}

	if common.Debug {
		w.log.Printf("[DEBUG] Loaded %d files for program %s:\n%v\n\n",
			len(files),
			files)
	}

	// Before I create a new playlist, I should check if the playlist
	// already exists and use that. Shouldn't be that hard, but tedious.

	if playlistNames, err = w.getPlaylistNames(); err != nil {
		return
	}

	for i, name := range playlistNames {
		if name == p.Title {
			w.log.Printf("[DEBUG] Playlist %s is already open (%d)\n",
				p.Title,
				i)
			obj.Call(objMethod(audInterface, methPlaylistSetActive), 0, int32(i))
			goto PLAY
		}
	}

	fileNames = make([]string, len(files))

	for i, f := range files {
		fileNames[i] = "file://" + f.Path
	}

	call = obj.Call(objMethod(audInterface, methPlaylistOpenFiles), dbus.FlagNoReplyExpected, fileNames)

	time.Sleep(time.Millisecond * 250)

	if call.Err != nil {
		msg = fmt.Sprintf("Failed to add files to playlist: %s",
			call.Err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
		return
	}

	call = obj.Call(
		objMethod(audInterface, methPlaylistRename),
		dbus.FlagNoReplyExpected,
		p.Title,
	)

PLAY:
	call = obj.Call(objMethod(audInterface, methPlay), dbus.FlagNoReplyExpected)

	time.Sleep(time.Millisecond * 250)

	if call.Err != nil {
		msg = fmt.Sprintf("Failed to tell the player to play: %s",
			call.Err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
		return
	}

	w.log.Printf("[TRACE] Done. The player should now be playing %s\n", p.Title)

	// TODO Jump to the current file and position!
	var fid = p.CurFile
	for idx, f := range files {
		if f.ID == fid {
			w.log.Printf("[DEBUG] Jump to file %d (%q)\n",
				f.ID,
				f.Title)
			obj.Call(
				objMethod(audInterface, methPlaylistJump),
				dbus.FlagNoReplyExpected,
				uint32(idx))
			time.Sleep(time.Millisecond * 250)
			w.log.Printf("[DEBUG] Seek to position %s\n",
				time.Second*time.Duration(f.Position))
			obj.Call(
				objMethod(audInterface, methSeek),
				dbus.FlagNoReplyExpected,
				uint32(f.Position)*1000)
			return
		}
	}

	w.log.Printf("[ERROR] Did not find current file %d\n",
		p.CurFile)
} // func (w *RWin) playerPlayProgram(p *objects.Program)

func (w *RWin) playerClearPlaylist() error {
	krylib.Trace()
	defer fmt.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	var (
		msg  string
		obj  = w.mbus.Object(objName, audPath)
		call *dbus.Call
	)

	if call = obj.Call(objMethod(audInterface, methPlaylistCreate), dbus.FlagNoReplyExpected); call == nil {
		msg = fmt.Sprintf("Failed to call method %s on Player", methPlaylistCreate)
		w.log.Printf("[ERROR] %s\n", msg)
		return errors.New(msg)
	} else if call.Err != nil {
		w.log.Printf("[ERROR] Error calling method %s on player: %s\n",
			methPlaylistCreate,
			call.Err.Error())
		return call.Err
	}

	return nil
} // func (w *RWin) playerClearPlaylist() error

// I thought I could listen for Signals from my player to notice when the
// track changes or something like that, BUT it turns out VLC has no
// useful Signals to deliver at all.
// But subscribing to signals is not that trivial, hence I leave this
// commented-out method here for future reference.

func (w *RWin) registerSignal() {
	w.log.Printf("[TRACE] Subscribing to signals on DBus\n")
	w.mbus.BusObject().Call(
		objName,
		0,
		"type='signal',path='/org/mpris/MediaPlayer2/Player/Seeked',interface='org.mpris.MediaPlayer2.Player',sender='org.mpris.MediaPlayer2.audacious'")

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

func (w *RWin) getPlaylistPosition() (int, error) {
	var (
		pos uint32
		err error
		obj = w.mbus.Object(objName, audPath)
	)

	err = obj.Call(objMethod(audInterface, methPlaylistPosition), 0).Store(&pos)
	if err != nil {
		var msg = fmt.Sprintf("Cannot query playlist position: %s",
			err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
		return 0, err
	}

	return int(pos), err
} // func (w *RWin) getPlaylistPosition() (int, error)

func (w *RWin) getSongFilename(pos int) (string, error) {
	var (
		err         error
		uriStr, msg string
		obj         = w.mbus.Object(objName, audPath)
	)

	if err = obj.Call(objMethod(audInterface, methFilename), 0, uint32(pos)).Store(&uriStr); err != nil {
		msg = fmt.Sprintf("Cannot query filename for position %d: %s",
			pos,
			err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
		return "", err
	}

	var fileURI *url.URL

	if fileURI, err = url.Parse(uriStr); err != nil {
		msg = fmt.Sprintf("Cannot parse file URL (%s): %s",
			uriStr,
			err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
		return "", err
	}

	return fileURI.Path, nil
} // func (w *RWin) getSongFilename(pos int) (string, error)

func (w *RWin) getSongPosition() (int64, error) {
	var (
		err error
		msg string
		pos uint32
		obj = w.mbus.Object(objName, audPath)
	)

	if err = obj.Call(objMethod(audInterface, methFilePosition), 0).Store(&pos); err != nil {
		msg = fmt.Sprintf("Error querying playback position: %s",
			err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
		return 0, err
	}

	return int64(pos) / 1000, nil
} // func (w *RWin) getSongPosition() (int64, error)

func (w *RWin) getPlaylistCount() (int32, error) {
	var (
		err error
		msg string
		cnt int32
		obj = w.mbus.Object(objName, audPath)
	)

	if err = obj.Call(objMethod(audInterface, methPlaylistCnt), 0).Store(&cnt); err != nil {
		msg = fmt.Sprintf("Cannot query number of playlists: %s",
			err.Error())
		w.log.Printf("[ERROR] %s\n", msg)
		w.displayMsg(msg)
	}

	return cnt, err
} // func (w *RWin) getPlaylistCount() (int32, error)

func (w *RWin) getPlaylistNames() ([]string, error) {
	var (
		err error
		msg string
		cnt int32
		obj = w.mbus.Object(objName, audPath)
	)

	if cnt, err = w.getPlaylistCount(); err != nil {
		return nil, err
	}

	var (
		idx   int32
		names = make([]string, cnt)
	)

	for idx = 0; idx < cnt; idx++ {
		var name string
		obj.Call(objMethod(audInterface, methPlaylistSetActive), 0, idx)
		if err = obj.Call(objMethod(audInterface, methPlaylistGetName), 0).Store(&name); err != nil {
			msg = fmt.Sprintf("Cannot query active playlist name: %s", err.Error())
			w.log.Printf("[ERROR] %s\n", msg)
			w.displayMsg(msg)
			return nil, err
		}

		names[idx] = name
	}

	return names, nil
} // func (w *RWin) getPlaylistNames() ([]string, error)

////////////////////////////////////////////////////////////////////////////////
//////////// Helpers ///////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

func objMethod(intf, method string) string {
	return intf + "." + method
} // func objMethod(intf, method string) string

func idle(n int) {
	for i := 0; i < n; i++ {
		gtk.MainIterationDo(false)
	}
} // func idle(n int)

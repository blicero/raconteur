// /home/krylon/go/src/github.com/blicero/raconteur/main.go
// -*- mode: go; coding: utf-8; -*-
// Created on 30. 05. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2023-09-08 21:12:31 krylon>

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/ui"
)

func main() {
	var (
		err      error
		win      *ui.RWin
		clearLog bool
	)

	fmt.Printf("%s %s, built on %s starting up...\n",
		common.AppName,
		common.Version,
		common.BuildStamp.Format(common.TimestampFormat))

	flag.BoolVar(&clearLog, "clear", false, "Truncate the log file at startup")

	flag.Parse()

	if clearLog {
		os.Remove(common.LogPath) // nolint: errcheck
	}

	if win, err = ui.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating UI: %s\n", err.Error())
		os.Exit(1)
	}

	win.Run()

} // func main()

// /home/krylon/go/src/github.com/blicero/raconteur/main.go
// -*- mode: go; coding: utf-8; -*-
// Created on 30. 05. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-13 18:58:12 krylon>

package main

import (
	"fmt"
	"os"

	"github.com/blicero/raconteur/common"
	"github.com/blicero/raconteur/ui"
)

func main() {
	var (
		win *ui.RWin
		err error
	)

	fmt.Printf("%s %s, built on %s starting up...\n",
		common.AppName,
		common.Version,
		common.BuildStamp.Format(common.TimestampFormat))

	if win, err = ui.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating UI: %s\n", err.Error())
		os.Exit(1)
	}

	win.Run()

} // func main()

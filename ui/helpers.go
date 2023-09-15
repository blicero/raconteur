// /home/krylon/go/src/github.com/blicero/raconteur/ui/helpers.go
// -*- mode: go; coding: utf-8; -*-
// Created on 15. 09. 2023 by Benjamin Walkenhorst
// (c) 2023 Benjamin Walkenhorst
// Time-stamp: <2023-09-15 19:44:03 krylon>

package ui

import (
	"github.com/gotk3/gotk3/gtk"
)

type signed interface {
	int | int8 | int16 | int32 | int64 | float32 | float64
}

func abs[t signed](n t) t {
	if n < t(0) {
		return -n
	}

	return n
} // func abs[t signed](n t) t

// func abs(n int64) int64 {
// 	if n == math.MinInt32 {
// 		return 0
// 	} else if n < 0 {
// 		return -n
// 	} else {
// 		return n
// 	}
// } // func abs(n int64) int64

func objMethod(intf, method string) string {
	return intf + "." + method
} // func objMethod(intf, method string) string

func idle(n int) { // nolint: unused
	for i := 0; i < n; i++ {
		gtk.MainIterationDo(false)
	}
} // func idle(n int)

// /home/krylon/go/src/github.com/blicero/raconteur/objects/01_file_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 14. 06. 2022 by Benjamin Walkenhorst
// (c) 2022 Benjamin Walkenhorst
// Time-stamp: <2022-06-14 18:58:32 krylon>

package objects

import "testing"

func TestGetParentFolder(t *testing.T) {
	type testCase struct {
		f File
		p string
	}

	var cases = []testCase{
		testCase{
			f: File{
				Path: "/data/Files/audio/test/test01.mp3",
			},
			p: "test",
		},
	}

	for _, c := range cases {
		if p := c.f.GetParentFolder(); p != c.p {
			t.Errorf(`Unexpected result from GetParentFolder:
Expected:	%s
Got:		%s`,
				c.p,
				p)
		}
	}
} // func TestGetParentFolder(t *testing.T)

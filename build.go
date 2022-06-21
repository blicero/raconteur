// /home/krylon/go/src/ticker/build.go
// -*- mode: go; coding: utf-8; -*-
// Created on 01. 02. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2022-06-21 20:04:38 krylon>

// +build ignore

package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blicero/krylib"

	"github.com/hashicorp/logutils"
)

const logFile = "./dbg.build.log"
const lintCommand = "mygolint"

var logLevels = []logutils.LogLevel{
	"TRACE",
	"DEBUG",
	"INFO",
	"WARN",
	"ERROR",
	"CRITICAL",
	"CANTHAPPEN",
	"SILENT",
}

var orderedSteps = []string{
	"clean",
	"generate",
	"vet",
	"lint",
	"test",
	"build",
}

var candidates = map[string][]string{
	"generate": []string{
		"common",
		"logdomain",
		"db/query",
	},
	"test": []string{
		"db",
		"objects",
	},
	"vet": []string{
		"common",
		"db",
		"db/query",
		"logdomain",
		"objects",
	},
	"lint": []string{
		"common",
		"db",
		"db/query",
		"logdomain",
		"objects",
	},
}

// During the clean step, all files and folders that match any of these
// regular expressions is removed.
// If a directory is matched, it is removed recursively without
// looking at its content.
var cleanPatterns = []*regexp.Regexp{
	regexp.MustCompile("ffjson"),
	regexp.MustCompile("_string.go$"),
	regexp.MustCompile("_gen.go$"),
}

var errDone = errors.New("Done")
var verbose bool
var dbg *log.Logger

// nolint: gocyclo
func main() {
	var (
		workerCnt             int
		err                   error
		before, after, t1, t2 time.Time
		minLevel              = "DEBUG"
		stepsRaw              string
		steps                 map[string]bool
		stepList              []string
		raceDetect            bool
		lvlString             = make([]string, len(logLevels))
	)

	for i, l := range logLevels {
		lvlString[i] = string(l)
	}

	flag.IntVar(&workerCnt, "parallel", runtime.NumCPU(), "Number of concurrent build processes")
	flag.BoolVar(&verbose, "verbose", false, "Emit additional messages to aid in debugging")
	flag.StringVar(&minLevel, "loglevel", "DEBUG", fmt.Sprintf(`Log messages with a lower priority than this will be discarded.
Valid log levels are: %s
This flag is not case-sensitive.`, strings.Join(lvlString, ", ")))
	flag.StringVar(&stepsRaw, "steps", "all",
		fmt.Sprintf(`The operations to perform. To perform multiple operations, separate them by
commas WITHOUT SPACES IN BETWEEN, like so: 
        -steps=generate,vet,build
No matter what in what order the steps are in the command line, they will
always be performed in the following order:
%s
For the brevity, the single value "all" will perform all steps except clean.
This flag is not case-sensitive.`, strings.Join(orderedSteps, ", ")))
	flag.BoolVar(&raceDetect, "race", false, "Build with race detector enabled")

	flag.Parse()

	minLevel = strings.ToUpper(minLevel)
	stepsRaw = strings.ToLower(stepsRaw)

	stepsRaw = strings.TrimRight(stepsRaw, " \r\n")

	if !verbose {
		var def bool
		if _, def = os.LookupEnv("BUILD_VERBOSE"); def {
			fmt.Println("XXX Turning on verbose mode due to environment variable BUILD_VERBOSE")
			minLevel = "TRACE"
			verbose = true
			time.Sleep(5)
		}
	}

	if err = initLog(minLevel); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializnig log: %s\n",
			err.Error())
		os.Exit(1)
	}

	if stepsRaw == "all" {
		stepList = orderedSteps[:]
	} else if stepList = strings.Split(stepsRaw, ","); stepList == nil {
		dbg.Printf("[ERROR] Invalid operations list: %s\n",
			stepsRaw)
		os.Exit(1)
	}

	steps = make(map[string]bool)

	for _, op := range stepList {
		dbg.Printf("[TRACE] I will run go %s\n",
			op)
		steps[op] = true
	}

	if n := runtime.NumCPU(); n < workerCnt {
		dbg.Printf("[INFO] Requested worker count %d is larger than the number of CPUs (%d)\n",
			workerCnt,
			n)
		workerCnt = n
	}

	dbg.Printf("[INFO] Using %d worker goroutines\n",
		workerCnt)

	before = time.Now()
	defer func() {
		after = time.Now()
		duration := after.Sub(before)
		dbg.Printf("[INFO] Total runtime of build.go was %s\n",
			duration)
	}()

	if steps["clean"] {
		dbg.Println("[INFO] Cleaning up generated files")
		if err = cleanup(); err != nil {
			dbg.Printf("[ERROR] Error while cleaning up: %s\n",
				err.Error())
			os.Exit(1)
		}
		// os.Exit(0)
	}

	dbg.Printf("[DEBUG] orderedSteps = %s\n", orderedSteps)
	for idx, s := range orderedSteps {
		dbg.Printf("[TRACE] Do we run go %s? %b\n",
			s,
			steps[s])

		if !steps[s] {
			dbg.Printf("[TRACE] Step %s was not requested, I skip it.\n",
				s)
			continue
		} else if s == "build" || s == "clean" {
			// clean is always done first, build is always done last
			continue
		}

		dbg.Printf("[INFO] Running go %s\n", s)
		t1 = time.Now()
		if err = dispatch(s, workerCnt); err != nil {
			dbg.Printf("[ERROR] Error in step %d (%s): %s\n",
				idx+1,
				s,
				err.Error())
			os.Exit(1)
		}
		t2 = time.Now()
		dbg.Printf("[INFO] %s took %s\n",
			s,
			t2.Sub(t1))
	}

	if steps["build"] {
		var output []byte

		dbg.Println("[INFO] Building raconteur\n")

		// Put aside a possibly existing binary
		if err = backupExecutable(); err != nil {
			dbg.Printf("[ERROR] Cannot backup existing executable: %s\n",
				err.Error())
			os.Exit(1)
		}

		// Build the program itself:
		var sWorkerCnt = strconv.FormatInt(int64(workerCnt), 10)
		// var cmd = exec.Command("go", "build", "-v", "-p", sWorkerCnt)
		// The -tags flag is required so the build will succeed on Debian.
		var args = []string{"build", "-v", "-tags", "pango_1_42,gtk_3_22", "-p", sWorkerCnt}

		if raceDetect && ((runtime.GOOS == "linux" || runtime.GOOS == "freebsd") && runtime.GOARCH == "amd64") {
			dbg.Println("[INFO] Building with race detection enabled.")
			args = append(args, "-race")
		}

		var cmd = exec.Command("go", args...)
		if output, err = cmd.CombinedOutput(); err != nil {
			dbg.Printf("[ERROR] Error building raconteur: %s\n%s\n",
				err.Error(),
				output)
			os.Exit(1)
		}
		println("Build was successful.")
	}
} // func main()

// nolint: gocyclo
func dispatch(op string, workers int) error {
	if l := len(candidates[op]); l < workers {
		workers = l
	}

	if op == "lint" {
		if _, e := exec.LookPath(lintCommand); e != nil {
			dbg.Printf("[INFO] Skipping %s: %s\n",
				lintCommand,
				e.Error())
			return nil
		}
	}

	var (
		result      error
		err         error
		idx, runCnt int
		wg          sync.WaitGroup
		errq        = make(chan error, workers*2)
		pkgq        = make(chan string)
		ticker      = time.NewTicker(time.Second * 5)
	)
	defer ticker.Stop()

	dbg.Printf("[TRACE] Run go %s\n", op)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker(i+1, op, pkgq, errq, &wg)
		runCnt++
	}

LOOP:
	for {
		select {
		case pkgq <- candidates[op][idx]:
			dbg.Printf("[TRACE] Package #%d (%s) has been dispatched.\n",
				idx+1,
				candidates[op][idx])
			idx++
			if idx >= len(candidates[op]) {
				break LOOP
			}

		case err = <-errq:
			runCnt--
			if err != errDone {
				dbg.Printf("[ERROR] %s\n",
					err.Error())
				result = err
				break LOOP
			}
		case <-ticker.C:
			// do nothing
		}
	}

	ticker.Stop()
	close(pkgq)
	wg.Wait()

	ticker = time.NewTicker(time.Millisecond * 250)

	// errq might contain sveral error values, so process those as well.
	for runCnt > 0 {
		select {
		case err = <-errq:
			runCnt--
			if err != errDone {
				result = err
				dbg.Printf("[ERROR] %s\n", err.Error())
			}

		case <-ticker.C:
			// This is just to make sure we do not block forever.
		}
	}

	dbg.Println("[TRACE] Dispatcher is done.")

	return result
} // func dispatch(op string, workers int) error

func worker(n int, op string, pkgq <-chan string, errq chan<- error, wg *sync.WaitGroup) {
	var (
		pkg string
		err error
		cmd *exec.Cmd
	)
	defer wg.Done()

	for folder := range pkgq {
		pkg = "github.com/blicero/raconteur/" + folder
		dbg.Printf("[TRACE] Worker %d call %s on %s\n",
			n,
			op,
			folder)

		var (
			errbuf bytes.Buffer
			errw   = bufio.NewWriter(&errbuf)
			outbuf bytes.Buffer
			outw   = bufio.NewWriter(&outbuf)
		)

		if op == "lint" {
			cmd = exec.Command(lintCommand, pkg)
			// cmd = exec.Command(
			// 	lintCommand,
			// 	"run",
			// 	pkg)
		} else if op == "test" {
			if runtime.GOOS == "openbsd" || runtime.GOARCH == "386" || runtime.GOARCH == "arm" {
				cmd = exec.Command("go", op, "-v", "-timeout", "30m", pkg)
			} else {
				cmd = exec.Command("go", op, "-v", "-timeout", "30m", "-race", pkg)
			}
		} else {
			cmd = exec.Command("go", op, "-v", pkg)
		}

		cmd.Stdout = outw
		cmd.Stderr = errw
		err = cmd.Run()

		if err != nil || !cmd.ProcessState.Success() {
			fmt.Printf("ERROR in %s on %s:\n%s\n%s\n",
				op, folder, outbuf.String(), errbuf.String())
			fmt.Fprintln(os.Stderr, errbuf.String())
			var result = fmt.Errorf("ERROR %sing %s: %s",
				op,
				folder,
				err.Error())
			errq <- result
			return
		}
	}

	dbg.Printf("[TRACE] Worker %d is done\n", n)
	errq <- errDone
} // func worker(n int, op string, pkgq <-chan string, errq chan<- error, wg *sync.WaitGroup)

func cleanup() error {
	var err error
	if err = filepath.Walk(".", visitor); err != nil {
		dbg.Printf("[ERROR] Error cleaning up: %s\n",
			err.Error())
		return err
	}
	return nil
} // func cleanup()

func visitor(path string, info os.FileInfo, incoming error) error {
	var err error
	if incoming != nil {
		return incoming
	}

	for _, re := range cleanPatterns {
		if re.MatchString(path) {
			dbg.Printf("[DEBUG] Cleanup %s\n",
				path)
			if info.IsDir() {
				if err = os.RemoveAll(path); err != nil {
					return err
				}
				return filepath.SkipDir
			}

			return os.Remove(path)
		}
	}

	return nil
} // func visitor(path string, info *os.FileInfo, incoming error) error

func initLog(min string) error {
	var (
		err    error
		fh     *os.File
		writer io.Writer
		// Trailing space because Logger does not seem to insert one
		// between fields of the line.
		logName = "raconteur.build "
	)

	// fmt.Printf("Creating Logger with minLevel = %q\n",
	// 	min)
	// fmt.Printf("LogLevels = %s\n", logLevels)

	if fh, err = os.OpenFile(logFile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644); err != nil {
		return fmt.Errorf("Error opening log file %s: %s\n",
			logFile,
			err.Error())
	}

	writer = io.MultiWriter(os.Stdout, fh)

	filter := &logutils.LevelFilter{
		Levels:   logLevels,
		MinLevel: logutils.LogLevel(min),
		Writer:   writer,
	}

	// fmt.Printf("Logger.MinLevel: %q\n",
	// 	filter.MinLevel)

	dbg = log.New(filter, logName, log.Ldate|log.Ltime|log.Lshortfile)
	return nil
} // func initLog() error

func backupExecutable() error {
	const (
		execPath   = "raconteur"
		backupPath = "bak.raconteur"
	)
	var (
		exists bool
		err    error
	)

	if exists, err = krylib.Fexists(execPath); err != nil {
		return err
	} else if !exists {
		return nil
	} else if exists, err = krylib.Fexists(backupPath); err != nil {
		return err
	} else if exists {
		if err = os.Remove(backupPath); err != nil {
			return err
		}
	}

	if err = os.Rename(execPath, backupPath); err != nil {
		return err
	}

	return nil
} // func backupExecutable() error

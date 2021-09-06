// /home/krylon/go/src/github.com/blicero/raconteur/common/common.go
// -*- mode: go; coding: utf-8; -*-
// Created on 06. 09. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2021-09-06 18:41:52 krylon>

package common

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/raconteur/logdomain"
	"github.com/hashicorp/logutils"
	uuid "github.com/odeke-em/go-uuid"
)

//go:generate ./build_time_stamp.pl

// AppName is the name under which the application identifies itself.
// Version is the version number.
// Debug, if true, causes the application to log additional messages and perform
// additional sanity checks.
// TimestampFormat is the default format for timestamp used throughout the
// application.
const (
	AppName                  = "Raconteur"
	Version                  = "0.0.1"
	Debug                    = true
	TimestampFormatMinute    = "2006-01-02 15:04"
	TimestampFormat          = "2006-01-02 15:04:05"
	TimestampFormatSubSecond = "2006-01-02 15:04:05.0000 MST"
	TimestampFormatDate      = "2006-01-02"
	TimestampFormatTime      = "15:04:05"
	NetName                  = "udp4"
)

// LogLevels are the names of the log levels supported by the logger.
var LogLevels = []logutils.LogLevel{
	"TRACE",
	"DEBUG",
	"INFO",
	"WARN",
	"ERROR",
	"CRITICAL",
	"CANTHAPPEN",
	"SILENT",
}

// PackageLevels defines minimum log levels per package.
var PackageLevels = make(map[logdomain.ID]logutils.LogLevel, len(LogLevels))

func init() {
	for _, id := range logdomain.AllDomains() {
		PackageLevels[id] = MinLogLevel
	}
} // func init()

var tildeRe = regexp.MustCompile(`^~`)

// MinLogLevel is the minimum level a log message must
// have to be written out to the log.
// This value is configurable to reduce log verbosity
// in regular use.
var MinLogLevel logutils.LogLevel = "TRACE"

// DoTrace causes the log level to be lowered to TRACE when set.
var DoTrace = true

// BaseDir is the folder where all application-specific files are stored.
// It defaults to $HOME/.Kuang2.d
var BaseDir = filepath.Join(
	krylib.GetHomeDirectory(),
	fmt.Sprintf(".%s.d", strings.ToLower(AppName)))

// LogPath is the filename of the log file.
var LogPath = filepath.Join(BaseDir, fmt.Sprintf("%s.log", strings.ToLower(AppName)))

// DbPath is the filename of the database.
var DbPath = filepath.Join(BaseDir, fmt.Sprintf("%s.db", strings.ToLower(AppName)))

// InitApp performs some basic preparations for the application to run.
// Currently, this means creating the BaseDir folder.
func InitApp() error {
	var err error

	if err = os.Mkdir(BaseDir, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("Error creating BaseDir %s: %s", BaseDir, err.Error())
	}

	LogPath = filepath.Join(BaseDir, fmt.Sprintf("%s.log", strings.ToLower(AppName)))
	DbPath = filepath.Join(BaseDir, fmt.Sprintf("%s.db", strings.ToLower(AppName)))

	return nil
} // func InitApp() error

// SetBaseDir sets the application's base directory. This should only be
// done during initialization.
// Once the log file and the database are opened, this
// is useless at best and opens a world of confusion at worst, so this function
// should only be called at the very beginning of the program.
func SetBaseDir(path string) error {
	if tildeRe.MatchString(path) {
		path = tildeRe.ReplaceAllString(path, krylib.GetHomeDirectory())
	}

	fmt.Printf("Setting BASE_DIR to %s\n", path)

	BaseDir = path
	LogPath = filepath.Join(BaseDir, fmt.Sprintf("%s.log", strings.ToLower(AppName)))
	DbPath = filepath.Join(BaseDir, fmt.Sprintf("%s.db", strings.ToLower(AppName)))

	var (
		err error
		msg string
	)

	if err = InitApp(); err != nil {
		msg = fmt.Sprintf("Error initializing application environment: %s\n",
			err.Error())
		fmt.Println(msg)
		return errors.New(msg)
	}

	return nil
} // func SetBaseDir(path string)

// GetLogger tries to create a named logger instance and return it.
// If the directory to hold the log file does not exist, try to create it.
func GetLogger(domain logdomain.ID) (*log.Logger, error) { // nolint: interfacer
	var (
		err     error
		logfile *os.File
		logName = fmt.Sprintf("%s.%s ",
			AppName,
			domain.String())
	)

	if err = InitApp(); err != nil {
		return nil, fmt.Errorf("Error initializing application environment: %s", err.Error())
	}

	if logfile, err = os.OpenFile(LogPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600); err != nil {
		msg := fmt.Sprintf("Error opening log file: %s\n", err.Error())
		fmt.Println(msg)
		return nil, errors.New(msg)
	}

	var (
		writer io.Writer
	)

	if Debug {
		writer = io.MultiWriter(os.Stdout, logfile)
	} else {
		writer = io.MultiWriter(logfile)
	}

	var lvl = PackageLevels[domain]

	filter := &logutils.LevelFilter{
		Levels:   LogLevels,
		MinLevel: lvl,
		Writer:   writer,
	}

	logger := log.New(filter, logName, log.Ldate|log.Ltime|log.Lshortfile)
	return logger, nil
} // func GetLogger(name string) (*log.Logger, error)

// GetLoggerStdout returns a Logger that will log to stdout AND the log file.
func GetLoggerStdout(domain logdomain.ID) (*log.Logger, error) { // nolint: interfacer
	var err error

	if err = InitApp(); err != nil {
		return nil, fmt.Errorf("Error initializing application environment: %s", err.Error())
	}

	var (
		logfile *os.File
		writer  io.Writer
		lvl     logutils.LogLevel
		logName = fmt.Sprintf("%s.%s ",
			AppName,
			domain.String())
	)

	if logfile, err = os.OpenFile(LogPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600); err != nil {
		msg := fmt.Sprintf("Error opening log file: %s\n", err.Error())
		fmt.Println(msg)
		return nil, errors.New(msg)
	}

	writer = io.MultiWriter(os.Stdout, logfile)

	lvl = PackageLevels[domain]

	filter := &logutils.LevelFilter{
		Levels:   LogLevels,
		MinLevel: lvl,
		Writer:   writer,
	}

	logger := log.New(filter, logName, log.Ldate|log.Ltime|log.Lshortfile)
	return logger, nil
} // func GetLoggerStdout(name string) (*log.Logger, error)

// GetUUID returns a randomized UUID
func GetUUID() string {
	return uuid.NewRandom().String()
} // func GetUUID() string

// TimeEqual returns true if the two timestamps are less than one second apart.
func TimeEqual(t1, t2 time.Time) bool {
	var delta = t1.Sub(t2)

	if delta < 0 {
		delta = -delta
	}

	return delta < time.Second
} // func TimeEqual(t1, t2 time.Time) bool

// GetChecksum computes the SHA512 checksum of the given data.
func GetChecksum(data []byte) (string, error) {
	var err error
	var hash = sha512.New()

	if _, err = hash.Write(data); err != nil {
		fmt.Fprintf( // nolint: errcheck
			os.Stderr,
			"Error computing checksum: %s\n",
			err.Error(),
		)
		return "", err
	}

	var checkSumBinary = hash.Sum(nil)
	var checkSumText = fmt.Sprintf("%x", checkSumBinary)

	return checkSumText, nil
} // func getChecksum(data []byte) (string, error)

// Copyright 2013, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
vtworker is the main program to run a worker job.

It has two modes: single command or interactive.
- in single command, it will start the job passed in from the command line,
  and exit.
- in interactive mode, use a web browser to start an action.
*/
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/golang/glog"
	"github.com/youtube/vitess/go/exit"
	"github.com/youtube/vitess/go/vt/logutil"
	"github.com/youtube/vitess/go/vt/servenv"
	"github.com/youtube/vitess/go/vt/tabletmanager/tmclient"
	"github.com/youtube/vitess/go/vt/topo"
	"github.com/youtube/vitess/go/vt/worker"
	"github.com/youtube/vitess/go/vt/wrangler"
	"golang.org/x/net/context"
)

var (
	cell = flag.String("cell", "", "cell to pick servers from")
)

func init() {
	servenv.RegisterDefaultFlags()
}

var (
	// global wrangler object we'll use
	wr *wrangler.Wrangler

	// mutex is protecting all the following variables
	// 3 states here:
	// - no job ever ran (or reset was run): currentWorker is nil,
	// currentContext/currentCancelFunc is nil, lastRunError is nil
	// - one worker running: currentWorker is set,
	//   currentContext/currentCancelFunc is set, lastRunError is nil
	// - (at least) one worker already ran, none is running atm:
	//   currentWorker is set, currentContext is nil, lastRunError
	//   has the error returned by the worker.
	currentWorkerMutex  sync.Mutex
	currentWorker       worker.Worker
	currentMemoryLogger *logutil.MemoryLogger
	currentContext      context.Context
	currentCancelFunc   context.CancelFunc
	lastRunError        error
)

// signal handling, centralized here
func installSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		// we got a signal, notify our modules
		currentWorkerMutex.Lock()
		defer currentWorkerMutex.Unlock()
		if currentCancelFunc != nil {
			currentCancelFunc()
		}
	}()
}

// setAndStartWorker will set the current worker.
// We always log to both memory logger (for display on the web) and
// console logger (for records / display of command line worker).
func setAndStartWorker(wrk worker.Worker) (chan struct{}, error) {
	currentWorkerMutex.Lock()
	defer currentWorkerMutex.Unlock()
	if currentWorker != nil {
		return nil, fmt.Errorf("A worker is already in progress: %v", currentWorker)
	}

	currentWorker = wrk
	currentMemoryLogger = logutil.NewMemoryLogger()
	currentContext, currentCancelFunc = context.WithCancel(context.Background())
	lastRunError = nil
	done := make(chan struct{})
	wr.SetLogger(logutil.NewTeeLogger(currentMemoryLogger, logutil.NewConsoleLogger()))

	// one go function runs the worker, changes state when done
	go func() {
		// run will take a long time
		log.Infof("Starting worker...")
		err := wrk.Run(currentContext)

		// it's done, let's save our state
		currentWorkerMutex.Lock()
		currentContext = nil
		currentCancelFunc = nil
		lastRunError = err
		currentWorkerMutex.Unlock()
		close(done)
	}()

	return done, nil
}

func main() {
	defer exit.Recover()

	flag.Parse()
	args := flag.Args()

	servenv.Init()
	defer servenv.Close()

	ts := topo.GetServer()
	defer topo.CloseServers()

	// The logger will be replaced when we start a job.
	wr = wrangler.New(logutil.NewConsoleLogger(), ts, tmclient.NewTabletManagerClient(), 30*time.Second)
	if len(args) == 0 {
		// In interactive mode, initialize the web UI to choose a command.
		initInteractiveMode()
	} else {
		// In single command mode, just run it.
		if err := runCommand(args); err != nil {
			log.Error(err)
			exit.Return(1)
		}
	}
	installSignalHandlers()
	initStatusHandling()

	servenv.RunDefault()
}

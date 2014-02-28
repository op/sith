// Copyright 2013 Örjan Persson
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package sith is the dark side of Spotify Core.
package sith

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	"github.com/codegangsta/martini"
	"github.com/martini-contrib/encoder"
	"github.com/op/go-libspotify/spotify"
	"github.com/op/go-logging"
)

var (
	prog = filepath.Base(os.Args[0])
	log  = logging.MustGetLogger(prog)
)

var (
	appKeyPath = flag.String("key", "spotify_appkey.key", "path to app.key")
	username   = flag.String("username", "o.p", "spotify username")
	password   = flag.String("password", "", "spotify password")
	port       = flag.Int("port", 8107, "HTTP port interface")
	color      = flag.Bool("color", true, "output log in colors")
)

// Run is the main entry point for this program.
func Run() {
	flag.Parse()
	setupLogging()

	// TODO there's a limitation with libspotify, we can only have one logged in
	//      user per process. maybe we should create a layer that spawns a new
	//      process for each session required and have a small layer between?
	//      that's why this is currently called a bridge. it doesn't do much
	//      right now.
	bridge := newBridge(newSession())
	app := &application{}

	root := resourcePath()

	m := martini.New()
	m.Use(martini.Static(
		filepath.Join(root, "html"),
	))
	m.Use(func(c martini.Context, w http.ResponseWriter) {
		c.MapTo(encoder.JsonEncoder{}, (*encoder.Encoder)(nil))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	})
	m.Map(bridge)

	// Exposed API methods
	router := martini.NewRouter()
	router.Get("/search", app.search)
	m.Action(router.Handle)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Info("Starting up HTTP interface at %s", addr)
	server := http.Server{
		Addr:    addr,
		Handler: m,
	}
	server.ListenAndServe()

	// go signalHandler(api, server)

	// TODO add and try to enforce use of TLS
	log.Debug("Starting up HTTP interface on %d", *port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start HTTP interface: %s", err)
	}
}

func setupLogging() {
	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logBackend.Color = *color

	logging.SetFormatter(logging.MustStringFormatter("%{time:2006-01-02T15:04:05.000} %{module} %{message}"))
	logging.SetBackend(logBackend)
}

func signalHandler(bridge *bridge, server *http.Server) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)
	for {
		select {
		case <-signals:
			// server.StopAccepting()
			bridge.Stop()
			return
		}
	}
}

// resourcePath returns the path to the directory which contains the
// application resources.
func resourcePath() string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic("failed to find package folder")
	}
	dir := filepath.Join(filepath.Dir(file), "..")

	// Verify that the path actually is what we believe it is.
	if _, err := os.Stat(filepath.Join(dir, "html")); err != nil {
		if os.IsNotExist(err) {
			// TODO add support for keeping resources in eg /usr/share
			panic("package installed in the file system not implemented")
		}
		panic(err)
	}
	return dir
}

// newSession creates a new libspotify session.
func newSession() *spotify.Session {
	appKey, err := ioutil.ReadFile(*appKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	// TODO select better cache locations
	session, err := spotify.NewSession(&spotify.Config{
		ApplicationKey:   appKey,
		ApplicationName:  prog,
		CacheLocation:    "tmp",
		SettingsLocation: "tmp",
	})

	// TODO move control of the session into the API
	credentials := spotify.Credentials{
		Username: *username,
		Password: *password,
	}
	remember := false
	if err = session.Login(credentials, remember); err != nil {
		log.Fatal(err)
	}
	return session
}

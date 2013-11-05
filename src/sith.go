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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ngmoco/falcore"
	"github.com/ngmoco/falcore/compression"
	"github.com/ngmoco/falcore/static_file"
	"github.com/op/go-libspotify"
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

	auth := newAuth()
	api := newApi(auth, newSession())

	root := resourcePath()
	router := falcore.NewPathRouter()
	router.AddMatch(`^/$`, &static_file.Filter{
		BasePath:   filepath.Join(root, "html", "index.html"),
		PathPrefix: "/",
	})
	router.AddMatch(`^/s/`, &static_file.Filter{
		BasePath:   filepath.Join(root, "html"),
		PathPrefix: "/s",
	})
	router.AddMatch(`^/search$`, apiRequest(api, func(req *falcore.Request) (reply, error) {
		return api.session(req.HttpRequest).search(req.HttpRequest)
	}))

	pipeline := falcore.NewPipeline()
	pipeline.RequestDoneCallback = falcore.NewRequestFilter(
		func(req *falcore.Request) *http.Response {
			req.Trace()
			return nil
		},
	)
	pipeline.Upstream.PushBack(router)
	pipeline.Downstream.PushBack(compression.NewFilter(nil))

	server := falcore.NewServer(*port, pipeline)
	server.Addr = fmt.Sprintf("127.0.0.1:%d", *port)

	go signalHandler(api, server)

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
	falcore.SetLogger(&FalcoreLogger{logging.MustGetLogger("falcor")})
}

func signalHandler(api *api, server *falcore.Server) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)
	for {
		select {
		case <-signals:
			server.StopAccepting()
			api.Stop()
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

// getRelativePathToBin returns the relative path from the package directory to
// the current executed binary.
func getRelativePathToBin(root string) string {
	bin, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(err)
	}
	// FIXME very go version specific and will break
	if strings.Contains(bin, filepath.Join(".", "go-build")[1:]) {
		bin, err = filepath.Abs(filepath.Dir(flag.Arg(0)))
		if err != nil {
			panic(err)
		}
	}
	var rel string
	if rel, err = filepath.Rel(bin, root); err != nil {
		panic(err)
	}
	return rel
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

// reply is the interface required to create the API response. We expose
// this as an interface to unify how success and errors are treated.
type reply interface {
	Data() interface{}
	StatusCode() int
}

type apiReply struct {
	status int
	data   interface{}
}

func (r *apiReply) Data() interface{} {
	return r.data
}

func (r *apiReply) StatusCode() int {
	if r.status == 0 {
		return 200
	}
	return r.status
}

// apiRequest returns a decorated falcore.RequestFilter which will
// automatically adapt the reply or error to a JSON response.

// Return detailed errors only when the error implements the reply
// interface. Details are not leaked for unhandled errors.
func apiRequest(api *api, handler func(*falcore.Request) (reply, error)) falcore.RequestFilter {
	return falcore.NewRequestFilter(func(req *falcore.Request) *http.Response {
		// api.begin()
		// defer api.done()
		rep, err := handler(req)
		if err != nil {
			var ok bool
			if rep, ok = err.(reply); !ok {
				rep = newInternalServerError("unhandled error")
				log.Error(err.Error())
			} else {
				log.Warning(err.Error())
			}
		}
		return jsonResponse(req, rep)
	})
}

// jsonResponse converts a reply to JSON and sets the HTTP response up.
func jsonResponse(req *falcore.Request, rep reply) *http.Response {
	headers := make(http.Header)
	headers.Add("Content-Type", "application/json")

	// TODO add support for JSONP
	var encoded bytes.Buffer
	enc := json.NewEncoder(&encoded)

	// TODO handle errors in falcore
	if err := enc.Encode(rep.Data()); err != nil {
		panic(err)
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, encoded.Bytes(), "", "    "); err != nil {
		panic(err)
	}
	body := indented.String()

	return falcore.SimpleResponse(
		req.HttpRequest,
		rep.StatusCode(),
		headers,
		body,
	)
}

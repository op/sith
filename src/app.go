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

package sith

import (
	"net/http"
	"strings"
	"sync"
	"time"

	sp "github.com/op/go-libspotify"
	"github.com/op/go-logging"
)

// api is the peripheral between the router and libspotify. Some methods are
// exposed directly, and some are hidden behind an authorization context.
type api struct {
	au   *auth
	sess *sp.Session

	mu      sync.RWMutex
	cond    *sync.Cond
	running bool
	exit    chan struct{}
}

func newApi(auth *auth, session *sp.Session) *api {
	a := &api{
		au:   auth,
		sess: session,
	}
	a.cond = sync.NewCond(a.mu.RLocker())
	go a.events()
	return a
}

// auth returns an authorization context.
func (a *api) auth(req *http.Request) *authContext {
	var ctx *authContext

	// TODO when adding support for logout/login, make sure no other requests
	//      are outstanding and execute the login alone
	oauth_token := req.FormValue("oauth_token")

	// TODO verify token
	// TODO setup context from token
	if oauth_token == "xxx" {
		ctx = &authContext{}
		ctx.scopes.search = true
	}

	return ctx
}

// session returns a session context which indicates that if the requesting
// user has access to the Spotify session we currently have. If there's no
// Spotify session available, these calls will wait until one is available.
func (a *api) session(req *http.Request) *sessionContext {
	ctx := a.auth(req)

	// TODO wait with timeout and not indefinitley
	a.sync()

	// TODO verify that the token user have permission to the logged in
	//      user in libspotify

	return &sessionContext{ctx, a.sess}
}

// sync tries to synchronize any call to first make sure we have a working
// session object to the Spotify backend.
func (a *api) sync() {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for !a.running {
		a.cond.Wait()
	}
}

// freeze marks the session as not beeing available for the moment.
func (a *api) freeze() {
	log.Debug("Freezing...")
	a.mu.Lock()
	defer a.mu.Unlock()
	a.running = false
	log.Debug("Frozen.")
}

// thaw unfreezes the session and makes it available again.
func (a *api) thaw() {
	log.Debug("Thawing...")
	a.mu.Lock()
	defer a.mu.Unlock()
	a.running = true
	a.cond.Broadcast()
	log.Debug("Thawed.")
}

func (a *api) Stop() {
	a.sess.Logout()
	a.exit <- struct{}{}
}

func (a *api) events() {
	var stopping bool

	for {
		select {
		case err := <-a.sess.LoginUpdates():
			time.Sleep(2 * time.Second)
			a.thaw()
			if err != nil {
				log.Error("Login? %s", err)
			}
			log.Info("Interface now available at http://localhost:%d/", *port)
		case <-a.sess.LogoutUpdates():
			a.freeze()
			log.Warning("Logged out. Interface thawed.")
			if stopping {
				return
			}
		case message := <-a.sess.LogMessages():
			a.log(message)
		case <-time.After(200 * time.Millisecond):
			select {
			case <-a.exit:
				stopping = true
			default:
			}
		}
	}
}

func (a *api) log(m *sp.LogMessage) {
	var (
		fmt  = "%s"
		args = []interface{}{m.Message}
	)

	// Split away the line number from the module. Not very interesting.
	var module = m.Module
	s := strings.SplitN(module, ":", 2)
	if len(s) == 2 {
		module = s[0]
	}

	logger := logging.MustGetLogger(module)

	switch m.Level {
	case sp.LogFatal:
		logger.Critical(fmt, args...)
	case sp.LogError:
		logger.Error(fmt, args...)
	case sp.LogWarning:
		logger.Warning(fmt, args...)
	case sp.LogInfo:
		logger.Info(fmt, args...)
	case sp.LogDebug:
		logger.Debug(fmt, args...)
	default:
		panic("unhandled log level")
	}
}

// authContext is an authorized session to the web application.
type authContext struct {
	scopes scopes
}

func (a *authContext) requireScopes(s *scopes) error {
	if a == nil {
		// TODO make 401 or 403, depending on auth error
		err := newUnauthorizedError("authorization required")
		err.Param = "oauth_token"
		return err
	}
	if s.search && !a.scopes.search {
		return newForbiddenError("insufficient scope")
	}
	return nil
}

type Track struct {
	Uri  string `json:"uri"`
	Name string `json:"name"`
}

func newTrack(t *sp.Track) *Track {
	return &Track{t.Link().String(), t.Name()}
}

type Album struct {
	Uri  string `json:"uri"`
	Name string `json:"name"`
}

func newAlbum(a *sp.Album) *Album {
	return &Album{a.Link().String(), a.Name()}
}

type Artist struct {
	Uri  string `json:"uri"`
	Name string `json:"name"`
}

func newArtist(a *sp.Artist) *Artist {
	return &Artist{a.Link().String(), a.Name()}
}

type SearchResult struct {
	Artists []*Artist `json:"artists"`
	Albums  []*Album  `json:"albums"`
	Tracks  []*Track  `json:"tracks"`
}

// sessionContext represents a fully logged in Spotify session in libspotify
// and that the requester have access to the session.
type sessionContext struct {
	*authContext
	sess *sp.Session
}

// search queries the Spotify catalogue with the given query.
func (s *sessionContext) search(req *http.Request) (reply, error) {
	requiredScopes := &scopes{search: true}
	if err := s.requireScopes(requiredScopes); err != nil {
		return nil, err
	}

	// TODO validate input
	query := req.FormValue("query")

	// TODO generalize paging
	page := 1
	limit := 10
	offset := (page - 1) * limit

	// TODO make options
	artists := true
	albums := true
	tracks := true

	spec := sp.SearchSpec{offset, limit}
	opts := sp.SearchOptions{}
	if artists {
		opts.Artists = spec
	}
	if albums {
		opts.Albums = spec
	}
	if tracks {
		opts.Tracks = spec
	}

	log.Debug("Searching %s...", query)
	search, err := s.sess.Search(query, &opts)
	if err != nil {
		return nil, err
	}
	search.Wait()

	var result SearchResult
	if artists {
		result.Artists = make([]*Artist, 0, search.Artists())
		for i := 0; i < search.Artists(); i++ {
			artist := search.Artist(i)
			artist.Wait()
			result.Artists = append(result.Artists, newArtist(artist))
		}
	}
	if albums {
		result.Albums = make([]*Album, 0, search.Albums())
		for i := 0; i < search.Albums(); i++ {
			album := search.Album(i)
			album.Wait()
			result.Albums = append(result.Albums, newAlbum(album))
		}
	}
	if tracks {
		result.Tracks = make([]*Track, 0, search.Tracks())
		for i := 0; i < search.Tracks(); i++ {
			track := search.Track(i)
			track.Wait()
			result.Tracks = append(result.Tracks, newTrack(track))
		}
	}
	return &apiReply{200, &result}, nil
}

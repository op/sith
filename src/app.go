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

	"github.com/martini-contrib/encoder"
	"github.com/op/go-logging"
	"github.com/op/go-libspotify/spotify"
)

type bridge struct {
	sess *spotify.Session

	mu      sync.RWMutex
	cond    *sync.Cond
	running bool
	exit    chan struct{}
}

func newBridge(session *spotify.Session) *bridge {
	b := &bridge{
		sess: session,
	}
	b.cond = sync.NewCond(b.mu.RLocker())
	go b.events()
	return b
}

// sync tries to synchronize any call to first make sure we have a working
// session object to the Spotify backend.
func (b *bridge) sync() {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for !b.running {
		b.cond.Wait()
	}
}

// freeze marks the session as not beeing available for the moment.
func (b *bridge) freeze() {
	log.Debug("Freezing...")
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running = false
	log.Debug("Frozen.")
}

// thaw unfreezes the session and makes it available again.
func (b *bridge) thaw() {
	log.Debug("Thawing...")
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running = true
	b.cond.Broadcast()
	log.Debug("Thawed.")
}

func (b *bridge) Stop() {
	b.sess.Logout()
	b.exit <- struct{}{}
}

func (b *bridge) events() {
	var stopping bool

	for {
		select {
		case err := <-b.sess.LoginUpdates():
			time.Sleep(2 * time.Second)
			b.thaw()
			if err != nil {
				log.Error("Login? %s", err)
			}
			log.Info("Interface now available at http://localhost:%d/", *port)
		case <-b.sess.LogoutUpdates():
			b.freeze()
			log.Warning("Logged out. Interface thawed.")
			if stopping {
				return
			}
		case message := <-b.sess.LogMessages():
			b.log(message)
		case <-time.After(200 * time.Millisecond):
			select {
			case <-b.exit:
				stopping = true
			default:
			}
		}
	}
}

func (b *bridge) log(m *spotify.LogMessage) {
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
	case spotify.LogFatal:
		logger.Critical(fmt, args...)
	case spotify.LogError:
		logger.Error(fmt, args...)
	case spotify.LogWarning:
		logger.Warning(fmt, args...)
	case spotify.LogInfo:
		logger.Info(fmt, args...)
	case spotify.LogDebug:
		logger.Debug(fmt, args...)
	default:
		panic("unhandled log level")
	}
}

type Track struct {
	Uri  string `json:"uri"`
	Name string `json:"name"`
}

func newTrack(t *spotify.Track) *Track {
	return &Track{t.Link().String(), t.Name()}
}

type Album struct {
	Uri  string `json:"uri"`
	Name string `json:"name"`
}

func newAlbum(a *spotify.Album) *Album {
	return &Album{a.Link().String(), a.Name()}
}

type Artist struct {
	Uri  string `json:"uri"`
	Name string `json:"name"`
}

func newArtist(a *spotify.Artist) *Artist {
	return &Artist{a.Link().String(), a.Name()}
}

type SearchResult struct {
	Artists []*Artist `json:"artists"`
	Albums  []*Album  `json:"albums"`
	Tracks  []*Track  `json:"tracks"`
}

type application struct {
}

// search queries the Spotify catalogue with the given query.
func (a *application) search(bridge *bridge, enc encoder.Encoder, req *http.Request) (int, []byte) {
	// requiredScopes := &scopes{search: true}
	// if err := s.requireScopes(requiredScopes); err != nil {
	// 	return nil, err
	// }

	bridge.sync()

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

	spec := spotify.SearchSpec{args.Offset(), args.Limit}
	opts := spotify.SearchOptions{}
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
	search, err := bridge.sess.Search(query, &opts)
	if err != nil {
		// TODO serialize pretty error
		log.Info(err.Error())
		return http.StatusInternalServerError, nil
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

	return http.StatusOK, encoder.Must(enc.Encode(result))
}

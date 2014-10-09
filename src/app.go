// Copyright 2013-2014 Örjan Persson
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
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/codegangsta/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/encoder"
	"github.com/op/go-libspotify/spotify"
	"github.com/op/go-logging"
)

type bridge struct {
	sess   *spotify.Session
	player player

	ew EventsWriter

	mu      sync.RWMutex
	cond    *sync.Cond
	running bool
	exit    chan struct{}
}

func newBridge(session *spotify.Session, ew EventsWriter) *bridge {
	b := &bridge{
		sess:   session,
		player: newPlayer(session, ew),
		ew:     ew,
	}
	b.cond = sync.NewCond(b.mu.RLocker())
	go b.processEvents()
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
	b.player.Close()
	b.sess.Logout()
	b.exit <- struct{}{}
}

func (b *bridge) processEvents() {
	var stopping bool

	var logLevels = map[spotify.LogLevel]string{
		spotify.LogFatal:   "fatal",
		spotify.LogError:   "error",
		spotify.LogWarning: "warning",
		spotify.LogInfo:    "info",
		spotify.LogDebug:   "debug",
	}

	for {
		select {
		case err := <-b.sess.LoggedInUpdates():
			time.Sleep(2 * time.Second)
			b.thaw()
			if err != nil {
				log.Error("Login? %s", err)
			}
			log.Info("Interface now available at http://localhost:%d/", *port)
			b.ew.SendEvent("logged-in", err)
		case <-b.sess.LoggedOutUpdates():
			b.freeze()
			log.Warning("Logged out. Interface thawed.")
			b.ew.SendEvent("logged-out", nil)
			if stopping {
				return
			}
		case err := <-b.sess.ConnectionErrorUpdates():
			log.Error("Connection error: %s", err)
			b.ew.SendEvent("connection-error", err)
		case msg := <-b.sess.MessagesToUser():
			log.Error("Message to user: %s", msg)
			b.ew.SendEvent("user-message", msg)
		case <-b.sess.PlayTokenLostUpdates():
			log.Warning("Play token lost.")
			b.ew.SendEvent("play-token-lost", nil)
		case message := <-b.sess.LogMessages():
			b.log(message)

			// Pass the message through using server sent message
			b.ew.SendEvent("log", struct {
				Time    int64  `json:"time"`
				Level   string `json:"level"`
				Module  string `json:"module"`
				Message string `json:"message"`
			}{
				message.Time.Unix(),
				logLevels[message.Level],
				message.Module,
				message.Message,
			})
		case <-b.sess.EndOfTrackUpdates():
			log.Info("End of track reached.")
			b.player.EndOfTrack()
			b.ew.SendEvent("track-end", nil)
		case err := <-b.sess.StreamingErrors():
			log.Info("Streaming errors: %s", err)
			b.ew.SendEvent("streaming-error", err)
		case <-b.sess.ConnectionStateUpdates():
			log.Info("Connection state updates available.")
			b.ew.SendEvent("connection-state", nil)
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
	URI  string `json:"uri"`
	Name string `json:"name"`
}

func newTrack(t *spotify.Track) *Track {
	return &Track{t.Link().String(), t.Name()}
}

type Album struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

func newAlbum(a *spotify.Album) *Album {
	return &Album{a.Link().String(), a.Name()}
}

type Artist struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

func newArtist(a *spotify.Artist) *Artist {
	return &Artist{a.Link().String(), a.Name()}
}

type Playlist struct {
	Id            string           `json:"id"`
	URI           string           `json:"uri"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	Collaborative bool             `json:"collaborative"`
	Subscribers   int              `json:"subscribers"`
	Owner         string           `json:"owner"`
	Tracks        []*PlaylistTrack `json:"tracks"`
}

type PlaylistTrack struct {
	UID  string `json:"uid"`
	URI  string `json:"uri"`
	Name string `json:"name"`
	User string `json:"user"`
	Time string `json:"time"`
}

func timeStr(t time.Time) string {
	return t.Format("2006-01-02T03:04:05Z")
}

func newPlaylistTrack(pt *spotify.PlaylistTrack) *PlaylistTrack {
	t := pt.Track()
	return &PlaylistTrack{
		playlistTrackUID(pt),
		t.Link().String(),
		t.Name(),
		pt.User().CanonicalName(),
		timeStr(pt.Time()),
	}
}

func newPlaylist(p *spotify.Playlist) *Playlist {
	// TODO do this in javascript, don't expose the "id"
	uri := p.Link().String()
	id := uri[strings.LastIndex(uri, ":")+1:]

	// TODO handle error
	owner, _ := p.Owner()

	return &Playlist{
		id,
		p.Link().String(),
		p.Name(),
		p.Description(),
		p.Collaborative(),
		p.NumSubscribers(),
		owner.CanonicalName(),
		nil,
	}
}

type PlaylistResult struct {
	Playlist *Playlist `json:"playlist"`
}

type PlaylistsResult struct {
	Playlists []*Playlist `json:"playlists"`
}

type SearchResult struct {
	URI        string `json:"uri"`
	DidYouMean string `json:"didyoumean"`

	Artists []*Artist `json:"artists"`
	Albums  []*Album  `json:"albums"`
	Tracks  []*Track  `json:"tracks"`
}

type application struct {
}

type searchArgs struct {
	RawPage  int    `form:"page" json:"page"`
	RawLimit int    `form:"limit" json:"limit"`
	Query    string `form:"query" json:"query" binding:"required"`
}

func (sa searchArgs) Validate(errors *binding.Errors, req *http.Request) {
	if sa.RawPage < 0 {
		// TODO
	}

	if sa.RawLimit < 0 {
		// TODO
	}
	fmt.Println("limit", sa.Limit())
}

func (sa searchArgs) Page() int {
	if sa.RawPage == 0 {
		return 1
	}
	return sa.RawPage
}

func (sa searchArgs) Limit() int {
	if sa.RawLimit == 0 {
		return 10
	}
	return sa.RawLimit
}

func (sa searchArgs) Offset() int {
	return (sa.Page() - 1) * sa.Limit()
}

// search queries the Spotify catalogue with the given query.
func (a *application) search(bridge *bridge, enc encoder.Encoder, args searchArgs) (int, []byte) {
	// requiredScopes := &scopes{search: true}
	// if err := s.requireScopes(requiredScopes); err != nil {
	// 	return nil, err
	// }

	bridge.sync()

	// TODO make options
	artists := true
	albums := true
	tracks := true

	spec := spotify.SearchSpec{args.Offset(), args.Limit()}
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

	log.Debug("Searching %s...", args.Query)
	search, err := bridge.sess.Search(args.Query, &opts)
	if err != nil {
		// TODO serialize pretty error
		log.Info(err.Error())
		return http.StatusInternalServerError, nil
	}
	search.Wait()

	result := SearchResult{
		URI:        search.Link().String(),
		DidYouMean: search.DidYouMean(),
	}
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

type playlistsArgs struct {
	RawPage  int `form:"page" json:"page"`
	RawLimit int `form:"limit" json:"limit"`
}

func (pa playlistsArgs) Page() int {
	if pa.RawPage == 0 {
		return 1
	}
	return pa.RawPage
}

func (pa playlistsArgs) Limit() int {
	if pa.RawLimit == 0 {
		return 10
	}
	return pa.RawLimit
}

func (pa playlistsArgs) Offset() int {
	return (pa.Page() - 1) * pa.Limit()
}

func (pa playlistsArgs) OffLimit() int {
	return pa.Offset() + pa.Limit()
}

// playlists returns the playlists for the user.
func (a *application) playlists(bridge *bridge, enc encoder.Encoder, args playlistsArgs) (int, []byte) {
	bridge.sync()

	playlists, err := bridge.sess.Playlists()
	if err != nil {
		log.Info(err.Error())
		return http.StatusInternalServerError, nil
	}

	playlists.Wait()

	r := PlaylistsResult{}

	// Make this more asynchronous? We probably don't won't to wait for all metadata.
	for i := args.Offset(); i < playlists.Playlists() && i < args.OffLimit(); i++ {
		switch playlists.PlaylistType(i) {
		case spotify.PlaylistTypePlaylist:
			playlist := playlists.Playlist(i)
			r.Playlists = append(r.Playlists, newPlaylist(playlist))
		// TODO
		case spotify.PlaylistTypeStartFolder:
		case spotify.PlaylistTypeEndFolder:
		case spotify.PlaylistTypePlaceholder:
		}
	}

	return http.StatusOK, encoder.Must(enc.Encode(r))
}

type playlistArgs struct {
	RawPage  int `form:"page" json:"page"`
	RawLimit int `form:"limit" json:"limit"`
}

func (pa playlistArgs) Page() int {
	if pa.RawPage == 0 {
		return 1
	}
	return pa.RawPage
}

func (pa playlistArgs) Limit() int {
	if pa.RawLimit == 0 {
		return 10
	}
	return pa.RawLimit
}

func (pa playlistArgs) Offset() int {
	return (pa.Page() - 1) * pa.Limit()
}

func (pa playlistArgs) OffLimit() int {
	return pa.Offset() + pa.Limit()
}

// playlist returns a specific playlist
func (a *application) playlist(bridge *bridge, enc encoder.Encoder, args playlistArgs, params martini.Params) (int, []byte) {
	bridge.sync()

	// TODO unescape?
	user := params["user"]
	id := params["id"]
	uri := fmt.Sprintf("spotify:user:%s:playlist:%s", user, id)

	link, err := bridge.sess.ParseLink(uri)
	if err != nil {
		log.Info(err.Error())
		return http.StatusInternalServerError, nil
	}
	if link.Type() != spotify.LinkTypePlaylist {
		return http.StatusBadRequest, nil
	}

	playlist, err := link.Playlist()
	if err != nil {
		log.Info(err.Error())
		return http.StatusInternalServerError, nil
	}

	playlist.Wait()

	r := PlaylistResult{newPlaylist(playlist)}

	for i := args.Offset(); i < playlist.Tracks() && i < args.OffLimit(); i++ {
		pt := playlist.Track(i)
		r.Playlist.Tracks = append(r.Playlist.Tracks, newPlaylistTrack(pt))
	}

	return http.StatusOK, encoder.Must(enc.Encode(r))
}

func (a *application) play(bridge *bridge, enc encoder.Encoder) (int, []byte) {
	bridge.sync()

	player := bridge.sess.Player()
	player.Play()

	return http.StatusOK, nil
}

func (a *application) pause(bridge *bridge, enc encoder.Encoder) (int, []byte) {
	bridge.sync()

	player := bridge.sess.Player()
	player.Pause()

	return http.StatusOK, nil
}

type loadArgs struct {
	Context string `form:"ctx"`
	Index   int    `form:"index"`
	URI     string `form:"uri"`
	Query   string `form:"query"`
}

func (a *application) load(bridge *bridge, enc encoder.Encoder, args loadArgs) (int, []byte) {
	bridge.sync()

	ctxLink, err := bridge.sess.ParseLink(args.Context)
	if err != nil {
		log.Info(err.Error())
		return http.StatusBadRequest, nil
	}
	var tracks trackList
	switch ctxLink.Type() {
	case spotify.LinkTypePlaylist:
		playlist, err := ctxLink.Playlist()
		if err != nil {
			return http.StatusInternalServerError, nil
		}
		playlist.Wait()
		tracks = &playlistTracks{playlist}
	case spotify.LinkTypeSearch:
		// TODO get offset and limit as arguments (offset of search)
		opts := spotify.SearchOptions{Tracks: spotify.SearchSpec{0, 50}}
		search, err := bridge.sess.Search(args.Query, &opts)
		if err != nil {
			return http.StatusInternalServerError, nil
		}
		search.Wait()
		tracks = &searchTracks{search}
	default:
		return http.StatusBadRequest, nil
	}

	if err = bridge.player.Play(tracks, args.Index); err != nil {
		return http.StatusInternalServerError, nil
	}

	return http.StatusOK, nil
}

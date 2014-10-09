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
	"errors"

	"github.com/op/go-libspotify/spotify"
)

const (
	playStateStopped = iota
	playStatePlaying = iota
)

var endOfContext = errors.New("end of context")

type trackInfo struct {
	UID   string
	Track *spotify.Track
}

// TODO hack, this interface doesn't make sense
type trackList interface {
	URI() string
	Len() int
	Get(int) (trackInfo, error)
}

type playlistTracks struct {
	playlist *spotify.Playlist
}

func (pt *playlistTracks) URI() string {
	return pt.playlist.Link().String()
}

func (pt *playlistTracks) Len() int {
	return pt.playlist.Tracks()
}

func (pt *playlistTracks) Get(n int) (trackInfo, error) {
	playlistTrack := pt.playlist.Track(n)
	uid := playlistTrackUID(playlistTrack)
	return trackInfo{uid, playlistTrack.Track()}, nil
}

type searchTracks struct {
	search *spotify.Search
}

func (st *searchTracks) URI() string {
	return st.search.Link().String()
}

func (st *searchTracks) Len() int {
	return st.search.Tracks()
}

func (st *searchTracks) Get(n int) (trackInfo, error) {
	// For the search view the URI is unique and enough
	track := st.search.Track(n)
	uid := track.Link().String()
	return trackInfo{uid, track}, nil
}

type playerContext struct {
	tracks trackList

	last  trackInfo
	index int
}

func (pc *playerContext) Next() (trackInfo, error) {
	i := pc.index

	// Find the current playing track and advance to next. Eg. playlists might
	// have been modified since last time, try to find the correct position.
	if pc.last.Track != nil {
		track, err := pc.tracks.Get(i)
		if err == nil && track.UID != pc.last.UID {
			for i = 0; i < pc.tracks.Len(); i++ {
				track, err = pc.tracks.Get(i)
				if err == nil && track.UID == pc.last.UID {
					break
				}
			}
			// As a last resort, assume we start from last index
			if i >= pc.tracks.Len() {
				i = pc.index
			}
		}
		i++
	}
	var err error
	pc.index = i % pc.tracks.Len()
	pc.last, err = pc.tracks.Get(pc.index)
	return pc.last, err
}

type player struct {
	session *spotify.Session

	// shuffle bool
	// repeat  bool

	queue chan *spotify.Track
	play  chan playerContext
	eot   chan bool
	quit  chan bool
}

func newPlayer(session *spotify.Session, ew EventsWriter) player {
	p := player{
		session: session,

		queue: make(chan *spotify.Track),
		play:  make(chan playerContext),
		eot:   make(chan bool),
		quit:  make(chan bool),
	}
	go p.loadTracks(ew)
	return p
}

func (p *player) Close() error {
	p.quit <- true
	return nil
}

func (p *player) Queue(uri string) error {
	link, err := p.session.ParseLink(uri)
	if err != nil {
		return err
	}

	track, err := link.Track()
	if err != nil {
		return err
	}

	p.queue <- track
	return nil
}

func (p *player) Play(tracks trackList, index int) error {
	p.play <- playerContext{tracks: tracks, index: index}
	return nil
}

func (p *player) EndOfTrack() {
	p.eot <- true
}

func (p *player) loadTracks(ew EventsWriter) {
	var queue []*spotify.Track
	var ctx playerContext

	player := p.session.Player()
	for {
		var newCtx bool
		select {
		case q := <-p.queue:
			queue = append(queue, q)
			continue
		case ctx = <-p.play:
			newCtx = true
		case <-p.eot:
			// do nothing
		case <-p.quit:
			return
		}

		var next trackInfo
		if !newCtx && len(queue) > 0 {
			next = trackInfo{"queue-uid", queue[0]}
			queue = queue[1:]
		} else {
			var err error
			println("getting next")
			next, err = ctx.Next()
			if err != nil {
				log.Error("Failed to fetch next track from context: %s", err.Error())
				continue
			}
			if next.Track == nil {
				player.Unload()
			}
		}

		// Release the queue array to make sure we don't grow memory indefinitley
		if len(queue) == 0 {
			queue = nil
		}

		if next.Track != nil {
			if err := player.Load(next.Track); err != nil {
				log.Error("Failed to load track: %s", err.Error())
				ew.SendEvent("play-track-failed", struct {
					URI string `json:"uri"`
				}{next.Track.Link().String()})
				continue
			}

			ew.SendEvent("play-track", struct {
				UID string `json:"uid"`
				URI string `json:"uri"`
			}{next.UID, next.Track.Link().String()})

			player.Play()
		}
	}
}

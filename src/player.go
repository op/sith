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

// TODO hack, this interface doesn't make sense
type trackList interface {
	Len() int
	Get(int) (*spotify.Track, error)
}

type playlistTracks struct {
	playlist *spotify.Playlist
}

func (pt *playlistTracks) Len() int {
	return pt.playlist.Tracks()
}

func (pt *playlistTracks) Get(n int) (*spotify.Track, error) {
	return pt.playlist.Track(n).Track(), nil
}

type searchTracks struct {
	search *spotify.Search
}

func (st *searchTracks) Len() int {
	return st.search.Tracks()
}

func (st *searchTracks) Get(n int) (*spotify.Track, error) {
	return st.search.Track(n), nil
}

type playerContext struct {
	tracks trackList
	offset int
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

func (p *player) Play(tracks trackList, offset int) error {
	p.play <- playerContext{tracks, offset}
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

		var next *spotify.Track
		if !newCtx && len(queue) > 0 {
			next = queue[0]
			queue = queue[1:]
		} else if ctx.tracks != nil {
			var err error
			index := ctx.offset % ctx.tracks.Len()
			next, err = ctx.tracks.Get(index)
			ctx.offset++
			if err != nil {
				log.Error("Failed to fetch next track from context: %s", err.Error())
				player.Pause()
				player.Unload()
				continue
			}
		}

		// Release the queue array to make sure we don't grow memory indefinitley
		if len(queue) == 0 {
			queue = nil
		}

		if next != nil {
			// TODO bubble any errors on up
			if err := player.Load(next); err != nil {
				log.Error("Failed to load track: %s", err.Error())
				ew.SendLink("play-track-failed", next.Link())
				continue
			}

			ew.SendLink("play-track", next.Link())
			player.Play()
		}
	}
}

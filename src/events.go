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
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/antage/eventsource"
	"github.com/op/go-libspotify/spotify"
)

const (
	esIdleTimeout = 30 * time.Minute
	esTimeout     = 5 * time.Second
)

type EventsWriter struct {
	es eventsource.EventSource

	// TODO base on timestamp?
	sequenceId int
}

// TODO this should not be kept here?

func NewEventsWriter() EventsWriter {
	esSettings := eventsource.Settings{
		CloseOnTimeout: false,
		IdleTimeout:    esIdleTimeout,
		Timeout:        esTimeout,
	}
	es := eventsource.New(&esSettings, nil)
	return EventsWriter{es, 0}
}

func (ew *EventsWriter) Close() error {
	ew.es.Close()
	return nil
}

func (ew *EventsWriter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ew.es.ServeHTTP(w, r)
}

func (ew *EventsWriter) SendEvent(event string, data interface{}) error {
	ew.sequenceId++
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	ew.es.SendEventMessage(string(bytes), event, strconv.Itoa(ew.sequenceId))
	return nil
}

func (ew *EventsWriter) SendLink(event string, link *spotify.Link) error {
	return ew.SendEvent(event, struct {
		URI string `json:"uri"`
	}{link.String()})
}

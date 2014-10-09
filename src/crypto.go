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
	"bytes"
	"crypto/sha1"
	"encoding/hex"

	"github.com/op/go-libspotify/spotify"
)

func playlistTrackUID(pt *spotify.PlaylistTrack) string {
	var data bytes.Buffer

	data.WriteString(pt.User().CanonicalName())
	data.WriteString(pt.Time().String())
	data.WriteString(pt.Track().Link().String())

	sum := sha1.Sum(data.Bytes())
	return hex.EncodeToString(sum[:])
}

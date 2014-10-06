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
	"code.google.com/p/portaudio-go/portaudio"
	"github.com/op/go-libspotify/spotify"
)

var (
	// audioBufferSize is the number of delivered data from libspotify before
	// we start rejecting it to deliver any more.
	audioBufferSize = 16

	// audioFormat is the internal format expected from libspotify.
	audioFormat = spotify.AudioFormat{
		spotify.SampleTypeInt16NativeEndian, 44100, 2,
	}
)

// audio wraps the delivered Spotify data into a single struct.
type audio struct {
	format spotify.AudioFormat
	frames []byte
}

// audioWriter takes audio from libspotify and outputs it through PortAudio.
type audioWriter struct {
	buffer chan audio
	stream *portaudio.Stream
}

// newAudioWriter creates a new audioWriter handler.
//
// This method is expected to only be called once during the lifetime of Go.
func newAudioWriter() (audioWriter, error) {
	pa := audioWriter{buffer: make(chan audio, audioBufferSize)}

	portaudio.Initialize()
	h, err := portaudio.DefaultHostApi()
	if err != nil {
		return pa, err
	}

	params := portaudio.LowLatencyParameters(nil, h.DefaultOutputDevice)
	params.Output.Channels = audioFormat.Channels
	params.SampleRate = float64(audioFormat.SampleRate)

	pa.stream, err = portaudio.OpenStream(params, pa.streamWriter())
	return pa, pa.stream.Start()
}

// Close stops and closes the audio stream and terminates PortAudio.
func (w *audioWriter) Close() error {
	if err := w.stream.Stop(); err != nil {
		return err
	}
	if err := w.stream.Close(); err != nil {
		return err
	}
	return portaudio.Terminate()
}

// WriteAudio implements the spotify.AudioWriter interface.
func (w *audioWriter) WriteAudio(format spotify.AudioFormat, frames []byte) int {
	select {
	case w.buffer <- audio{format, frames}:
		return len(frames)
	default:
		return 0
	}
}

// streamWriter reads data from the internal buffer and writes it into the
// internal portaudio buffer.
func (w *audioWriter) streamWriter() func([]int16) {
	var i int
	var buf audio
	return func(out []int16) {
		for j := 0; j < len(out); j++ {
			if i >= len(buf.frames) {
				buf = <-w.buffer
				if !audioFormat.Equal(buf.format) {
					panic("unexpected audio format")
				}
				i = 0
			}

			// Decode the incoming data which is expected to be 2 channels and
			// delivered as int16 in []byte, hence we need to convert it.
			out[j] = int16(buf.frames[i]) | int16(buf.frames[i+1])<<8
			i += 2
		}
	}
}

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

type audio struct {
	format spotify.AudioFormat
	frames []byte
}

type portAudio struct {
	buffer chan audio
}

func newPortAudio() *portAudio {
	return &portAudio{buffer: make(chan audio, 8)}
}

func (pa *portAudio) WriteAudio(format spotify.AudioFormat, frames []byte) int {
	select {
	case pa.buffer <- audio{format, frames}:
		return len(frames)
	default:
		return 0
	}
}

func (pa *portAudio) player() {
	out := make([]int16, 2048*2)

	// TODO clean up
	stream, err := portaudio.OpenDefaultStream(
		0,
		2,     // audio.format.Channels,
		44100, // float64(audio.format.SampleRate),
		len(out),
		&out,
	)
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	stream.Start()
	defer stream.Stop()

	// Decode the incoming data which is expected to be 2 channels and
	// delivered as int16 in []byte, hence we need to convert it.
	for audio := range pa.buffer {
		if len(audio.frames) != 2048*2*2 {
			println("AUDIO frames", len(audio.frames))
			// 	panic(fmt.Sprintf("unexpected: %d", len(audio.frames)))
		}

		j := 0
		for i := 0; i < len(audio.frames); i += 2 {
			out[j] = int16(audio.frames[i]) | int16(audio.frames[i+1])<<8
			j++
		}
		for j < 2048*2 {
			out[j] = 0
			j++
		}

		stream.Write()
	}
}

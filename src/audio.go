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
	"runtime"
	"sync"
	"time"

	"code.google.com/p/portaudio-go/portaudio"
	"github.com/op/go-libspotify/spotify"
)

var (
	// audioInputBufferSize is the number of delivered data from libspotify before
	// we start rejecting it to deliver any more.
	audioInputBufferSize = 8

	// audioOutputBufferSize is the maximum number of bytes to buffer before
	// passing it to PortAudio.
	audioOutputBufferSize = 8192

	// audioErrorDelay is the duration to delay between errors when setting up the
	// audio stream.
	audioErrorDelay = time.Second
)

// audio wraps the delivered Spotify data into a single struct.
type audio struct {
	format spotify.AudioFormat
	frames []byte
}

// audioWriter takes audio from libspotify and outputs it through PortAudio.
type audioWriter struct {
	input chan audio

	quit chan bool
	err  chan error
	wg   sync.WaitGroup
}

// newAudioWriter creates a new audioWriter handler.
func newAudioWriter() (audioWriter, error) {
	w := audioWriter{
		input: make(chan audio, audioInputBufferSize),
		quit:  make(chan bool, 1),
	}

	stream, err := newPortAudioStream()
	if err != nil {
		return w, err
	}

	w.wg.Add(1)
	go w.streamWriter(stream)
	return w, nil
}

// Close stops and closes the audio stream and terminates PortAudio.
func (w *audioWriter) Close() error {
	w.quit <- true
	w.wg.Wait()
	return nil
}

func (w *audioWriter) Error() <-chan error {
	return w.err
}

// WriteAudio implements the spotify.AudioWriter interface.
func (w *audioWriter) WriteAudio(format spotify.AudioFormat, frames []byte) int {
	select {
	case w.input <- audio{format, frames}:
		return len(frames)
	default:
		return 0
	}
}

// streamWriter reads data from the input buffer and writes it to the output
// portaudio buffer.
func (w *audioWriter) streamWriter(stream portAudioStream) {
	defer w.wg.Done()
	defer stream.Close()

	runtime.LockOSThread()
	buffer := make([]int16, audioOutputBufferSize)
	output := buffer[:]

	for {
		// Wait for input data or signal to quit.
		var input audio
		select {
		case input = <-w.input:
		case <-w.quit:
			return
		}

		// Initialize the audio stream based on the specification of the input format.
		err := stream.Stream(&output, input.format.Channels, input.format.SampleRate)
		if err != nil {
			select {
			case w.err <- err:
			default:
				log.Error("Failed to prepare audio stream: %s", err)
				time.Sleep(audioErrorDelay)
			}
			continue
		}

		// Decode the incoming data which is expected to be 2 channels and
		// delivered as int16 in []byte, hence we need to convert it.
		i := 0
		for i < len(input.frames) {
			j := 0
			for j < len(buffer) && i < len(input.frames) {
				buffer[j] = int16(input.frames[i]) | int16(input.frames[i+1])<<8
				j += 1
				i += 2
			}

			output = buffer[:j]
			stream.Write()
		}
	}
}

// portAudioStream manages the output stream through PortAudio when requirement
// for number of channels or sample rate changes.
type portAudioStream struct {
	device *portaudio.DeviceInfo
	stream *portaudio.Stream

	channels   int
	sampleRate int
}

// newPortAudioStream creates a new portAudioStream using the default output
// device found on the system. It will also take care of automatically
// initialise the PortAudio API.
func newPortAudioStream() (portAudioStream, error) {
	var s portAudioStream
	if err := portaudio.Initialize(); err != nil {
		return s, err
	}
	out, err := portaudio.DefaultHostApi()
	if err != nil {
		portaudio.Terminate()
		return s, err
	}
	s.device = out.DefaultOutputDevice
	return s, nil
}

// Close closes any open audio stream and terminates the PortAudio API.
func (s *portAudioStream) Close() error {
	if err := s.reset(); err != nil {
		portaudio.Terminate()
		return err
	}
	return portaudio.Terminate()
}

func (s *portAudioStream) reset() error {
	if s.stream != nil {
		if err := s.stream.Stop(); err != nil {
			return err
		}
		if err := s.stream.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Stream prepares the stream to go through the specified buffer, channels and
// sample rate, re-using any previously defined stream or setting up a new one.
func (s *portAudioStream) Stream(buffer *[]int16, channels int, sampleRate int) error {
	if s.stream == nil || s.channels != channels || s.sampleRate != sampleRate {
		if err := s.reset(); err != nil {
			return err
		}

		params := portaudio.HighLatencyParameters(nil, s.device)
		params.Output.Channels = channels
		params.SampleRate = float64(sampleRate)
		params.FramesPerBuffer = len(*buffer)

		stream, err := portaudio.OpenStream(params, buffer)
		if err != nil {
			return err
		}
		if err := stream.Start(); err != nil {
			stream.Close()
			return err
		}

		s.stream = stream
		s.channels = channels
		s.sampleRate = sampleRate
	}
	return nil
}

// Write pushes the data in the buffer through to PortAudio.
func (s *portAudioStream) Write() error {
	return s.stream.Write()
}

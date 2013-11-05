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
	"strings"

	"github.com/ngmoco/falcore"
	"github.com/op/go-logging"
)

type loggingFunc func(string, ...interface{})

// FIXME falcore.level should be exported.
var (
	falcoreFINEST   = int(falcore.FINEST)
	falcoreFINE     = int(falcore.FINE)
	falcoreDEBUG    = int(falcore.DEBUG)
	falcoreTRACE    = int(falcore.TRACE)
	falcoreINFO     = int(falcore.INFO)
	falcoreWARNING  = int(falcore.WARNING)
	falcoreERROR    = int(falcore.ERROR)
	falcoreCRITICAL = int(falcore.CRITICAL)
)

// FalcoreLogger adapts a logger to be compatible with Falcore.
type FalcoreLogger struct {
	Logger *logging.Logger
}

func (f *FalcoreLogger) Finest(arg0 interface{}, args ...interface{}) {
	f.log(falcoreFINEST, f.Logger.Debug, arg0, args...)
}

func (f *FalcoreLogger) Fine(arg0 interface{}, args ...interface{}) {
	f.log(falcoreFINE, f.Logger.Debug, arg0, args...)
}
func (f *FalcoreLogger) Debug(arg0 interface{}, args ...interface{}) {
	f.log(falcoreDEBUG, f.Logger.Debug, arg0, args...)
}
func (f *FalcoreLogger) Trace(arg0 interface{}, args ...interface{}) {
	f.log(falcoreTRACE, f.Logger.Debug, arg0, args...)
}
func (f *FalcoreLogger) Info(arg0 interface{}, args ...interface{}) {
	f.log(falcoreINFO, f.Logger.Info, arg0, args...)
}
func (f *FalcoreLogger) Warn(arg0 interface{}, args ...interface{}) error {
	return f.log(falcoreWARNING, f.Logger.Warning, arg0, args...)
}
func (f *FalcoreLogger) Error(arg0 interface{}, args ...interface{}) error {
	return f.log(falcoreERROR, f.Logger.Error, arg0, args...)
}
func (f *FalcoreLogger) Critical(arg0 interface{}, args ...interface{}) error {
	return f.log(falcoreCRITICAL, f.Logger.Critical, arg0, args...)
}

func (f *FalcoreLogger) log(level int, logf loggingFunc, arg0 interface{}, args ...interface{}) error {
	switch arg := arg0.(type) {
	case string:
		logf(arg, args...)
	case func() string:
		logf(arg())
	default:
		// TODO make nicer
		var a []interface{}
		a = append(a, arg0)
		a = append(a, args...)
		logf(strings.Repeat("%s ", len(a)), a...)
	}
	return nil
}

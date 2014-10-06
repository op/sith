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
	"net/http"
)

// TODO these are no longer used, but should be..
// apiError is the internal struct used to represent expected and handled
// errors.
type apiError struct {
	status int

	Code        string `json:"code"`
	Description string `json:"description"`
	Param       string `json:"param"`
}

func (e *apiError) Error() string {
	return e.Description
}

func (e *apiError) Data() interface{} {
	return &struct {
		Error *apiError `json:"error"`
	}{e}
}

func (e *apiError) StatusCode() int {
	if e.status == 0 {
		return 500
	}
	return e.status
}

func newForbiddenError(description string) *apiError {
	return &apiError{
		status:      http.StatusForbidden,
		Code:        "access_error",
		Description: description,
	}
}

func newUnauthorizedError(description string) *apiError {
	return &apiError{
		status:      http.StatusUnauthorized,
		Code:        "access_error",
		Description: description,
	}
}

func newInternalServerError(description string) *apiError {
	return &apiError{
		status:      http.StatusInternalServerError,
		Code:        "server_error",
		Description: description,
	}
}

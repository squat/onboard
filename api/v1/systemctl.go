// Copyright 2021 the Onboard authors
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

package v1

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type showResult string

const (
	successShowResult showResult = "success"
	successShowFailed showResult = "failed"
)

type showSubState string

const (
	deadShowFailed  showSubState = "dead"
	startShowResult showSubState = "start"
)

type showResponse struct {
	Result   showResult   `json:"result"`
	SubState showSubState `json:"subState"`
}

func systemctlShow(ctx context.Context, unit string) (*showResponse, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "show", unit, "--property", "Result", "--property", "SubState")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read command output: %w", err)
	}
	s := bufio.NewScanner(bytes.NewBuffer(out))
	var line string
	var parts []string
	var sr showResponse
	for s.Scan() {
		line = strings.TrimSpace(s.Text())
		if len(line) == 0 {
			continue
		}
		parts = strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "Result":
			sr.Result = showResult(parts[1])
		case "SubState":
			sr.SubState = showSubState(parts[1])
		}
	}
	return &sr, nil
}

func newSystemctlShowHandler(l log.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		unit := r.FormValue("unit")
		if unit == "" {
			msg := "received empty unit"
			level.Warn(l).Log("msg", msg)
			httpError(w, msg, http.StatusBadRequest)
			return
		}
		s, err := systemctlShow(r.Context(), unit)
		if err != nil {
			msg := "failed to get show unit status"
			level.Error(l).Log("msg", msg, "error", err.Error())
			httpError(w, msg, http.StatusBadRequest)
			return
		}
		buf, err := json.Marshal(s)
		if err != nil {
			msg := "failed to marshal response"
			level.Error(l).Log("msg", msg, "error", err.Error())
			httpError(w, msg, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(buf)
	}
}

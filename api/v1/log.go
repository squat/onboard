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
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

func journalctl(ctx context.Context, w io.Writer, matchers ...string) error {
	cmd := exec.CommandContext(ctx, "journalctl", append([]string{"--follow", "--output-fields=MESSAGE", "--output=json"}, matchers...)...)
	cmd.Stdout = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to follow journal: %w", err)
	}
	return nil
}

type logReader func(context.Context, io.Writer) error

func logReaderForMatcher(matchers ...string) logReader {
	return func(ctx context.Context, w io.Writer) error {
		return journalctl(ctx, w, matchers...)
	}
}

type sseWriter struct {
	http.Flusher
	io.Writer
}

func (s *sseWriter) Write(p []byte) (n int, err error) {
	defer s.Flush()
	return fmt.Fprintf(s.Writer, "data: %s\n\n", string(p))
}

func newLogHandler(l log.Logger, lr logReader) func(http.ResponseWriter, *http.Request) {
	return sseMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if err := lr(r.Context(), &sseWriter{w.(http.Flusher), w}); err != nil {
			msg := "failed to follow log stream"
			http.Error(w, msg, http.StatusInternalServerError)
			level.Error(l).Log("msg", msg, "error", err.Error())
			return
		}
	})
}

func sseMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			http.Error(w, "streaming is not supported", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		next(w, r)
	}
}

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

package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type roundTripperInstrumenter struct {
	requestCounter  *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func newRoundTripperInstrumenter(r prometheus.Registerer) *roundTripperInstrumenter {
	ins := &roundTripperInstrumenter{
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "client_api_requests_total",
				Help: "A counter for requests from the wrapped client.",
			},
			[]string{"code", "method", "client"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "request_duration_seconds",
				Help:    "A histogram of request latencies.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "client"},
		),
	}

	if r != nil {
		r.MustRegister(
			ins.requestCounter,
			ins.requestDuration,
		)
	}

	return ins
}

// NewRoundTripper wraps a HTTP RoundTripper with some metrics.
func (i *roundTripperInstrumenter) NewRoundTripper(name string, rt http.RoundTripper) http.RoundTripper {
	counter := i.requestCounter.MustCurryWith(prometheus.Labels{"client": name})
	duration := i.requestDuration.MustCurryWith(prometheus.Labels{"client": name})

	return promhttp.InstrumentRoundTripperCounter(counter,
		promhttp.InstrumentRoundTripperDuration(duration, rt),
	)
}

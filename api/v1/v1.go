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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/metalmatze/signal/server/signalhttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vishvananda/netlink"
)

func New(r prometheus.Registerer, l log.Logger, id, wlanInterface string, actions []func(map[string]string) error) http.Handler {
	hi := signalhttp.NewHandlerInstrumenter(r, []string{"handler"})
	m := http.NewServeMux()

	m.HandleFunc("/api/v1/log/systemd-networkd", hi.NewHandler(prometheus.Labels{"handler": "log-systemd-networkd"}, http.HandlerFunc(newLogHandler(l, logReaderForMatcher("SYSLOG_IDENTIFIER=systemd-networkd", fmt.Sprintf("INTERFACE=%s", wlanInterface))))))
	m.HandleFunc("/api/v1/log/wpa_supplicant", hi.NewHandler(prometheus.Labels{"handler": "log-systemd-networkd"}, http.HandlerFunc(newLogHandler(l, logReaderForMatcher(fmt.Sprintf("SYSLOG_IDENTIFIER=wpa_supplicant@%s", wlanInterface))))))
	m.HandleFunc("/api/v1/status/link", hi.NewHandler(prometheus.Labels{"handler": "status-link"}, http.HandlerFunc(newLinkHandler(l, wlanInterface))))
	m.HandleFunc("/api/v1/status/dns", hi.NewHandler(prometheus.Labels{"handler": "status-dns"}, http.HandlerFunc(newDNSHandler(l))))
	m.HandleFunc("/api/v1/status/systemd", hi.NewHandler(prometheus.Labels{"handler": "status-systemd"}, http.HandlerFunc(newSystemctlShowHandler(l))))
	m.HandleFunc("/api/v1/onboard", hi.NewHandler(prometheus.Labels{"handler": "onboard"}, http.HandlerFunc(newOnboardHandler(l, id, actions))))

	return m
}

func newOnboardHandler(l log.Logger, id string, actions []func(map[string]string) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "failed to read request"
			level.Error(l).Log("msg", msg, "error", err.Error())
			httpError(w, msg, http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		onboardRequest := make(map[string]string)
		if err := json.Unmarshal(body, &onboardRequest); err != nil {
			msg := "failed to unmarshal request"
			level.Error(l).Log("msg", msg, "error", err.Error())
			httpError(w, msg, http.StatusInternalServerError)
			return
		}

		for _, a := range actions {
			if err := a(onboardRequest); err != nil {
				msg := "failed to execute action"
				level.Error(l).Log("msg", msg, "error", err.Error())
				httpError(w, msg, http.StatusInternalServerError)
				return
			}
		}
	}
}

type linkResponse struct {
	Addresses []string `json:"addresses"`
	State     string   `json:"state"`
}

func newLinkHandler(l log.Logger, iface string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		li, err := netlink.LinkByName(iface)
		if err != nil {
			msg := "failed to find interface"
			level.Error(l).Log("msg", msg, "error", err.Error())
			httpError(w, msg, http.StatusInternalServerError)
			return
		}
		addressList, err := netlink.AddrList(li, netlink.FAMILY_ALL)
		if err != nil {
			msg := "failed to list addresses"
			level.Error(l).Log("msg", msg, "error", err.Error())
			httpError(w, msg, http.StatusInternalServerError)
			return
		}
		addresses := make([]string, 0, len(addressList))
		for _, a := range addressList {
			addresses = append(addresses, a.String())
		}
		buf, err := json.Marshal(linkResponse{
			Addresses: addresses,
			State:     li.Attrs().OperState.String(),
		})
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

func newDNSHandler(l log.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		h, _, err := net.SplitHostPort(r.FormValue("endpoint"))
		if err != nil {
			msg := "failed to parse endpoint"
			level.Error(l).Log("msg", msg, "error", err.Error())
			httpError(w, msg, http.StatusBadRequest)
			return
		}
		names, err := net.LookupHost(h)
		if err != nil {
			msg := "failed to lookup hostname"
			level.Error(l).Log("msg", msg, "error", err.Error())
			httpError(w, msg, http.StatusInternalServerError)
			return
		}
		if len(names) == 0 {
			msg := "found no addresses for host"
			level.Error(l).Log("msg", msg, "error", msg)
			httpError(w, msg, http.StatusInternalServerError)
			return
		}
	}
}

type jsonError struct {
	Error string `json:"error"`
}

func httpError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(jsonError{msg})
}

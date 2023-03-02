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
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/hashicorp/mdns"
	"github.com/metalmatze/signal/healthcheck"
	"github.com/metalmatze/signal/internalserver"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	flag "github.com/spf13/pflag"

	v1 "github.com/squat/onboard/api/v1"
)

type options struct {
	logLevel  level.Option
	logFormat string
	name      string

	id            string
	ipAddress     string
	wlanInterface string
	paths         []string
	cfg           *config

	server serverConfig
}

type serverConfig struct {
	listen         string
	listenInternal string
	healthcheckURL string
}

//go:embed static/build
var static embed.FS

func parseFlags() (*options, error) {
	opts := &options{}
	flag.StringVar(&opts.name, "debug.name", "onboard", "A name to add as a prefix to log lines.")
	logLevelRaw := flag.String("log.level", "info", "The log filtering level. Options: 'error', 'warn', 'info', 'debug'.")
	flag.StringVar(&opts.logFormat, "log.format", "logfmt", "The log format to use. Options: 'logfmt', 'json'.")
	flag.StringVar(&opts.server.listen, "web.listen", ":8080", "The address on which the public server listens.")
	flag.StringVar(&opts.server.listenInternal, "web.internal.listen", ":8081", "The address on which the internal server listens.")
	flag.StringVar(&opts.server.healthcheckURL, "web.healthchecks.url", "http://localhost:8080", "The URL against which to run healthchecks.")
	flag.StringVar(&opts.id, "id", "", "The ID for this device.")
	flag.StringVar(&opts.ipAddress, "ip-address", "10.0.0.1", "The IP address of the device running this process.")
	flag.StringVar(&opts.wlanInterface, "wlan-interface", "wlan0", "The name of the WLAN interface to configure.")
	flag.StringArrayVarP(&opts.paths, "config", "c", nil, "The path to the configuration file for Onboard. Can be specified multiple times to concatenate mutiple configuration files. Can be a glob, e.g. /path/to/configs/*.yaml. Files are processed in lexicographic order.")

	flag.Parse()

	switch *logLevelRaw {
	case "error":
		opts.logLevel = level.AllowError()
	case "warn":
		opts.logLevel = level.AllowWarn()
	case "info":
		opts.logLevel = level.AllowInfo()
	case "debug":
		opts.logLevel = level.AllowDebug()
	default:
		return nil, fmt.Errorf("unexpected log level: %s", *logLevelRaw)
	}

	var paths []string
	for _, path := range opts.paths {
		matches, err := filepath.Glob(path)
		if err != nil {
			return nil, fmt.Errorf("failed to find matches for path %q: %w", path, err)
		}
		paths = append(paths, matches...)
	}
	opts.cfg = &config{}
	sort.Slice(paths, func(i, j int) bool { return filepath.Base(paths[i]) < filepath.Base(paths[j]) })
	for _, path := range paths {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %q: %w", path, err)
		}
		c := &config{}
		if err := yaml.Unmarshal(data, c); err != nil {
			return nil, fmt.Errorf("failed to read YAML from file %q: %w", path, err)
		}
		opts.cfg.Actions = append(opts.cfg.Actions, c.Actions...)
		opts.cfg.Checks = append(opts.cfg.Checks, c.Checks...)
		opts.cfg.Values = append(opts.cfg.Values, c.Values...)
	}
	if err := opts.cfg.validate(); err != nil {
		return nil, err
	}

	return opts, nil
}

func main() {
	opts, err := parseFlags()
	if err != nil {
		stdlog.Fatal(err)
	}

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	if opts.logFormat == "json" {
		logger = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
	}

	logger = level.NewFilter(logger, opts.logLevel)

	if opts.name != "" {
		logger = log.With(logger, "name", opts.name)
	}

	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	defer level.Info(logger).Log("msg", "exiting")

	reg := prometheus.NewRegistry()
	//rti := newRoundTripperInstrumenter(reg)

	healthchecks := healthcheck.NewMetricsHandler(healthcheck.NewHandler(), reg)

	if opts.server.healthcheckURL != "" {
		// checks if server is up
		healthchecks.AddLivenessCheck("http",
			healthcheck.HTTPCheckClient(
				&http.Client{},
				opts.server.healthcheckURL,
				http.MethodGet,
				http.StatusNotFound,
				time.Second,
			),
		)
	}

	level.Info(logger).Log("msg", "starting onboard")
	var g run.Group
	{
		// Signal channels must be buffered.
		sig := make(chan os.Signal, 1)
		g.Add(func() error {
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			<-sig
			level.Info(logger).Log("msg", "caught interrupt")
			return nil
		}, func(_ error) {
			close(sig)
		})
	}
	{
		knownPaths := map[string]struct{}{
			"/":       {},
			"/submit": {},
		}
		j := []byte("{}")
		var err error
		actions := make([]func(map[string]string) error, 0, len(opts.cfg.Actions))
		for _, a := range opts.cfg.Actions {
			actions = append(actions, a.action())
		}
		for _, v := range opts.cfg.Values {
			knownPaths["/"+v.Name] = struct{}{}
		}
		j, err = json.Marshal(opts.cfg)
		if err != nil {
			stdlog.Fatal(err)
		}
		staticFS, err := fs.Sub(static, "static/build")
		if err != nil {
			stdlog.Fatal(err)
		}
		staticHandler := http.FileServer(http.FS(staticFS))
		v1Handler := v1.New(reg, logger, opts.id, opts.wlanInterface, actions)
		h := func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				v1Handler.ServeHTTP(w, r)
				return
			}
			if _, ok := knownPaths[r.URL.Path]; ok {
				index, err := staticFS.Open("index.html")
				if err != nil {
					panic(err)
				}
				indexHTML, err := ioutil.ReadAll(index)
				if err != nil {
					panic(err)
				}
				fmt.Fprint(w, strings.Replace(string(indexHTML), "configuration={}", "configuration="+string(j), 1))
			} else {
				staticHandler.ServeHTTP(w, r)
			}
		}
		s := http.Server{
			Addr:    opts.server.listen,
			Handler: http.HandlerFunc(h),
		}

		g.Add(func() error {
			level.Info(logger).Log("msg", "starting the HTTP server", "address", opts.server.listen)
			return s.ListenAndServe()
		}, func(err error) {
			level.Info(logger).Log("msg", "shutting down the HTTP server")
			_ = s.Shutdown(context.Background())
		})
	}
	{
		h := internalserver.NewHandler(
			internalserver.WithName("Internal - onboard API"),
			internalserver.WithHealthchecks(healthchecks),
			internalserver.WithPrometheusRegistry(reg),
			internalserver.WithPProf(),
		)

		s := http.Server{
			Addr:    opts.server.listenInternal,
			Handler: h,
		}

		g.Add(func() error {
			level.Info(logger).Log("msg", "starting internal HTTP server", "address", s.Addr)
			return s.ListenAndServe()
		}, func(err error) {
			_ = s.Shutdown(context.Background())
		})
	}
	{
		parts := strings.Split(opts.server.listen, ":")
		if len(parts) == 0 {
			stdlog.Fatal("invalid listening address")
		}
		port, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			stdlog.Fatal(err)
		}
		service, err := mdns.NewMDNSService(strings.TrimSpace(fmt.Sprintf("Onboard %s", opts.id)), "_http._tcp.", "local.", "onboard.local.", port, []net.IP{net.ParseIP(opts.ipAddress)}, []string{fmt.Sprintf("id=%s", opts.id)})
		if err != nil {
			stdlog.Fatal(err)
		}
		server, err := mdns.NewServer(&mdns.Config{Zone: service})
		if err != nil {
			stdlog.Fatal(err)
		}
		stop := make(chan struct{})
		g.Add(func() error {
			level.Info(logger).Log("msg", "starting mDNS server")
			<-stop
			return server.Shutdown()
		}, func(err error) {
			close(stop)
		})
	}

	if err := g.Run(); err != nil {
		stdlog.Fatal(err)
	}
}

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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
)

var validName = regexp.MustCompile(`^[a-zA-Z_]+[a-zA-Z0-9_-]*$`)
var validUnitName = regexp.MustCompile(`^([a-zA-Z0-9:._-]+@)?[a-zA-Z0-9:._-]+(\.service|\.socket|\.device|\.mount|\.automount|\.swap|\.target|\.path|\.timer|\.slice|\.scope)$`)

type SystemdCommand string

const (
	SystemdCommandStart   SystemdCommand = "start"
	SystemdCommandStop    SystemdCommand = "stop"
	SystemdCommandRestart SystemdCommand = "restart"
	SystemdCommandEnable  SystemdCommand = "enable"
	SystemdCommandDisable SystemdCommand = "disable"
)

type Action struct {
	Name    string         `json:"name"`
	File    *FileAction    `json:"file"`
	Systemd *SystemdAction `json:"systemd"`
}

func (a *Action) validate(cfg *config) error {
	var errs []string
	if len(a.Name) == 0 {
		return errors.New("action name cannot be empty")
	}
	if !validName.MatchString(a.Name) {
		errs = append(errs, fmt.Sprintf("action name %q does not match format %s", a.Name, validName.String()))
	}
	n := 0
	if a.File != nil {
		n++
		if err := a.File.validate(cfg.Values); err != nil {
			errs = append(errs, fmt.Sprintf("action %q: %v", a.Name, err))
		}
	}
	if a.Systemd != nil {
		n++
		if err := a.Systemd.validate(); err != nil {
			errs = append(errs, fmt.Sprintf("action %q: %v", a.Name, err))
		}
	}
	if n != 1 {
		errs = append(errs, fmt.Sprintf("action %q: exactly one of 'file' must be specified", a.Name))
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (a *Action) action() func(map[string]string) error {
	if a.File != nil {
		return a.File.action()
	}
	if a.Systemd != nil {
		return a.Systemd.action()
	}
	return nil
}

type FileAction struct {
	Path     string  `json:"path"`
	Value    *string `json:"value"`
	Template *string `json:"template"`
	t        *template.Template
}

func (f *FileAction) validate(values []*Value) error {
	var errs []string
	if len(f.Path) == 0 {
		return errors.New("file path cannot be empty")
	}
	n := 0
	if f.Value != nil {
		n++
		if len(*f.Value) == 0 {
			errs = append(errs, "file value must point at a defined value")
		}
		var found bool
		for _, v := range values {
			if v.Name == *f.Value {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf("file value %q was not found", *f.Value))
		}
	}
	if f.Template != nil {
		n++
		if len(*f.Template) == 0 {
			errs = append(errs, "file template cannot be empty")
		}
		t, err := template.New(f.Path).Parse(*f.Template)
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to parse template: %v", err))
		} else {
			f.t = t
		}
	}
	if n != 1 {
		errs = append(errs, "exactly one of 'file' or 'template' must be specified")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (f *FileAction) action() func(map[string]string) error {
	if f.Value != nil {
		return func(values map[string]string) error {
			return ioutil.WriteFile(f.Path, []byte(values[*f.Value]), 0644)
		}
	}
	if f.Template != nil {
		return func(values map[string]string) error {
			file, err := os.OpenFile(f.Path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to open file %q: %w", f.Path, err)
			}
			defer file.Close()
			if err := f.t.Execute(file, values); err != nil {
				return fmt.Errorf("failed to execute template: %w", err)
			}
			return nil
		}
	}
	return nil
}

type SystemdAction struct {
	Unit    string         `json:"unit"`
	Command SystemdCommand `json:"command"`
}

func (s *SystemdAction) validate() error {
	var errs []string
	if len(s.Unit) == 0 {
		errs = append(errs, "unit name cannot be empty")
	} else if !validUnitName.MatchString(s.Unit) {
		errs = append(errs, fmt.Sprintf("unit name %q does not match format %s", s.Unit, validUnitName.String()))
	}
	parts := strings.Split(s.Unit, "@")
	if len(parts[len(parts)-1]) > 256 {
		errs = append(errs, "unit name cannot exceed 256 characters")
	}

	switch s.Command {
	case SystemdCommandStart:
	case SystemdCommandStop:
	case SystemdCommandRestart:
	case SystemdCommandEnable:
	case SystemdCommandDisable:
	default:
		errs = append(errs, fmt.Sprintf("systemd command must be one of: %s", strings.Join([]string{string(SystemdCommandStart), string(SystemdCommandStop), string(SystemdCommandRestart), string(SystemdCommandEnable), string(SystemdCommandDisable)}, ",")))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (s *SystemdAction) action() func(map[string]string) error {
	return func(_ map[string]string) error {
		if err := exec.Command("systemctl", string(s.Command), s.Unit).Run(); err != nil {
			return fmt.Errorf("failed to execute systemd action: %w", err)
		}
		return nil
	}
}

type Check struct {
	Name        string        `json:"name"`
	Systemd     *SystemdCheck `json:"systemd"`
	GRPC        *GRPCCheck    `json:"gRPC"`
	DNS         *DNSCheck     `json:"dns"`
	Description string        `json:"description"`
}

func (c *Check) validate(cfg *config) error {
	var errs []string
	if len(c.Name) == 0 {
		return errors.New("check name cannot be empty")
	}
	if !validName.MatchString(c.Name) {
		errs = append(errs, fmt.Sprintf("check name %q does not match format %s", c.Name, validName.String()))
	}
	n := 0
	if c.Systemd != nil {
		n++
		if err := c.Systemd.validate(); err != nil {
			errs = append(errs, fmt.Sprintf("check %q: %v", c.Name, err))
		}
	}
	if c.GRPC != nil {
		n++
		if err := c.GRPC.validate(); err != nil {
			errs = append(errs, fmt.Sprintf("check %q: %v", c.Name, err))
		}
	}
	if c.DNS != nil {
		n++
		if err := c.DNS.validate(cfg.Values); err != nil {
			errs = append(errs, fmt.Sprintf("check %q: %v", c.Name, err))
		}
	}
	if n != 1 {
		errs = append(errs, fmt.Sprintf("check %q: exactly one of 'dns', 'gRPC', or 'systemd' must be specified", c.Name))
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

type SystemdCheck struct {
	Unit        string `json:"Unit"`
	Description string `json:"description"`
}

func (s *SystemdCheck) validate() error {
	if len(s.Unit) == 0 {
		return errors.New("unit name cannot be empty")
	}
	parts := strings.Split(s.Unit, "@")
	if len(parts[len(parts)-1]) > 256 {
		return errors.New("unit name cannot exceed 256 characters")
	}
	if !validUnitName.MatchString(s.Unit) {
		return fmt.Errorf("unit name %q does not match format %s", s.Unit, validUnitName.String())
	}
	return nil
}

type DNSCheck struct {
	Value string `json:"value"`
}

func (d *DNSCheck) validate(values []*Value) error {
	if len(d.Value) == 0 {
		return errors.New("DNS value must point at a defined value")
	}
	for _, v := range values {
		if v.Name == d.Value {
			return nil
		}
	}
	return fmt.Errorf("DNS value %q was not found", d.Value)
}

type GRPCCheck struct {
	Name   string `json:"name"`
	Socket string `json:"socket"`
}

func (g *GRPCCheck) validate() error {
	if len(g.Name) == 0 {
		return errors.New("gRPC name cannot be empty")
	}
	if !validName.MatchString(g.Name) {
		return fmt.Errorf("gRPC name %q does not match format %s", g.Name, validName.String())
	}
	return nil
}

type Value struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Secret      bool   `json:"secret"`
}

func (v *Value) validate() error {
	var errs []string
	if len(v.Name) == 0 {
		return errors.New("value name cannot be empty")
	}
	if !validName.MatchString(v.Name) {
		errs = append(errs, fmt.Sprintf("value name %q does not match format %s", v.Name, validName.String()))
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

type config struct {
	Actions []*Action `json:"actions"`
	Checks  []*Check  `json:"checks"`
	Values  []*Value  `json:"values"`
}

func (c *config) validate() error {
	if c.Actions == nil {
		c.Actions = []*Action{}
	}
	if c.Checks == nil {
		c.Checks = []*Check{}
	}
	if c.Values == nil {
		c.Values = []*Value{}
	}
	var errs []string
	actions := make(map[string]struct{})
	for _, a := range c.Actions {
		if err := a.validate(c); err != nil {
			errs = append(errs, err.Error())
		}
		if _, ok := actions[a.Name]; ok {
			errs = append(errs, fmt.Sprintf("action %q appears more than once", a.Name))
		} else {
			actions[a.Name] = struct{}{}
		}
	}
	checks := make(map[string]struct{})
	for _, ch := range c.Checks {
		if err := ch.validate(c); err != nil {
			errs = append(errs, err.Error())
		}
		if _, ok := checks[ch.Name]; ok {
			errs = append(errs, fmt.Sprintf("check %q appears more than once", ch.Name))
		} else {
			checks[ch.Name] = struct{}{}
		}
	}
	values := make(map[string]struct{})
	for _, v := range c.Values {
		if err := v.validate(); err != nil {
			errs = append(errs, err.Error())
		}
		if _, ok := values[v.Name]; ok {
			errs = append(errs, fmt.Sprintf("value %q appears more than once", v.Name))
		} else {
			values[v.Name] = struct{}{}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("configuration contains validation errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

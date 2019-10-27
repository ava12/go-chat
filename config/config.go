package config

import (
	"encoding/json"
	"io"
)

type layer interface {
	Section (name string, dst interface{}) error
}


type jsonLayer struct {
	sections map[string]json.RawMessage
}

func newJsonLayer () *jsonLayer {
	return &jsonLayer {make(map[string]json.RawMessage)}
}

func (jl *jsonLayer) Section (name string, dst interface{}) error {
	section, ack := jl.sections[name]
	if !ack {
		return nil
	}

	return json.Unmarshal(section, dst)
}


type Config struct {
	layers []layer
}

func New () *Config {
	return &Config {make([]layer, 0, 1)}
}

func (c *Config) Section (name string, dst interface{}) error {
	for _, layer := range c.layers {
		e := layer.Section(name, dst)
		if e != nil {
			return e
		}
	}

	return nil
}

func (c *Config) LoadJson (data []byte) error {
	layer := newJsonLayer()
	e := json.Unmarshal(data, &layer)
	if e != nil {
		return e
	}

	c.layers = append(c.layers, layer)
	return nil
}

func (c *Config) ReadJson (r io.Reader) error {
	layer := newJsonLayer()
	decoder := json.NewDecoder(r)
	e := decoder.Decode(&layer)
	if e != nil {
		return e
	}

	c.layers = append(c.layers, layer)
	return nil
}

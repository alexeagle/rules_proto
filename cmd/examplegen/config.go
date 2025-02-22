package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
)

type Config struct {
	Name        string
	Label       string
	TestOut     string
	MarkdownOut string
	WorkspaceIn string
	StripPrefix string
	Files       []string
}

// fromJSON constructs a Config struct from the given filename that contains a
// JSON.
func fromJSON(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	log.Printf("config: %+v", config)
	return &config, nil
}

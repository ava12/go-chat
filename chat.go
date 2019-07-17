package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"encoding/json"
	"path/filepath"
	"github.com/ava12/go-chat/server"
)

const (
	errArgs = iota + 1
	errConfig
	errServer
	errOther
)

type config struct {
	Addr string
	BaseDir string
	Dirs map[string]string
}

func main () {
	var e error
/*
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(errArgs)
	}
*/
	configName := "/home/alex/projects/go/src/github.com/ava12/go-chat/default.config.json"//os.Args[1]
	conf, e := readConfig(configName)
	stop(errConfig, e)

	cwd, e := os.Getwd()
	stop(errConfig, e)

	stop(errConfig, os.Chdir(filepath.Dir(configName)))
	s, e := newServer(conf)
	os.Chdir(cwd)
	stop(errServer, e)

	log.Println("starting")

	go goWaitForSignals(s)

	log.Println(s.Run())
	log.Println("stopping")

	os.Exit(0)
}

func stop (code int, e error) {
	if e == nil {
		return
	}

	fmt.Println(e.Error())
	os.Exit(code)
}

func printHelp () {
	fmt.Println("Usage is  chat <config.json>")
	fmt.Println("")
}

func readConfig (name string) (*config, error) {
	f, e := os.Open(name)
	if e != nil {
		return nil, e
	}

	de := json.NewDecoder(f)
	result := &config {}
	e = de.Decode(result)
	if e != nil {
		return nil, e
	}

	if result.BaseDir == "" {
		var dirname string
		dirname, e = filepath.Abs(name)
		result.BaseDir = filepath.Dir(dirname)
	}
	return result, e
}

func newServer (c *config) (*server.Server, error) {
	result := server.New()
	if c.Addr != "" {
		result.Addr = c.Addr
	}

	for url, path := range c.Dirs {
		path, e := filepath.Abs(path)
		if e != nil {
			return nil, e
		}

		result.AddFilePath(url, path)
	}

	return result, nil
}

func goWaitForSignals (s *server.Server) {
	signals := make (chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	<- signals
	log.Println("SIGINT")
	s.Stop()
}
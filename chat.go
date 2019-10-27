package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"github.com/ava12/go-chat/server"
	"github.com/ava12/go-chat/hub"
	"github.com/ava12/go-chat/config"
	access "github.com/ava12/go-chat/access/simple"
	proto "github.com/ava12/go-chat/proto/simple"
	room "github.com/ava12/go-chat/room/ram"
	session "github.com/ava12/go-chat/session/ram"
	user "github.com/ava12/go-chat/user/ram"
)

const (
	errArgs = iota + 1
	errConfig
	errServer
	errOther
)

func main () {
	var e error

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(errArgs)
	}

	configName := os.Args[1]
	conf, baseDir, e := readConfig(configName)
	stop(errConfig, e)

	cwd, e := os.Getwd()
	stop(errConfig, e)

	stop(errConfig, os.Chdir(baseDir))
	s, e := newServer(conf)
	os.Chdir(cwd)
	stop(errServer, e)

	s.Hub = hub.New(hub.NewMemStorage())
	s.Sessions = session.NewRegistry()
	s.Users = user.NewRegistry()
	s.Proto = proto.New(s.Hub, s.Users, room.NewRegistry(), access.NewAccessController())

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

func readConfig (name string) (*config.Config, string, error) {
	var e error
	f, e := os.Open(name)
	if e != nil {
		return nil, "", e
	}

	result := config.New()
	e = result.ReadJson(f)
	if e != nil {
		return nil, "", e
	}

	var baseDir string
	e = result.Section("BaseDir", &baseDir)
	if e != nil {
		return nil, "", e
	}

	if baseDir == "" {
		absName, e := filepath.Abs(name)
		if e != nil {
			return nil, "", e
		}

		baseDir = filepath.Dir(absName)
	} else {
		baseDir, e = filepath.Abs(baseDir)
	}

	return result, baseDir, e
}

func newServer (c *config.Config) (*server.Server, error) {

	result, e := server.New(c)
	return result, e
}

func goWaitForSignals (s *server.Server) {
	signals := make (chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	<- signals
	log.Println("SIGINT")
	s.Stop()
}

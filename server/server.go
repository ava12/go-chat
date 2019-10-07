package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava12/go-chat/conn"
	"github.com/ava12/go-chat/conn/ws"
	"github.com/ava12/go-chat/fserv"
	"github.com/ava12/go-chat/hub"
	"github.com/ava12/go-chat/proto/simple"
	ramSession "github.com/ava12/go-chat/session/ram"
	ramUser "github.com/ava12/go-chat/user/ram"
	simpleAC "github.com/ava12/go-chat/access/simple"
	ramRoom "github.com/ava12/go-chat/room/ram"
)

const (
	RefreshQueues = 2
	RefreshPeriod = time.Minute

	WsPath     = "/ws"
	WhoamiPath = "/whoami"
	LoginPath  = "/login"
	LogoutPath = "/logout"

	DefaultAddr        = ":8080"
	DefaultSessionName = "sid"
	DefaultSessionTtl  = 365 * 86400
)

type whoamiRec struct {
	Success bool        `json:"success"`
	User    interface{} `json:"user,omitempty"`
}

type refreshItem struct {
	conn    conn.Conn
	session *ramSession.Session
}

func logRequest(r *http.Request, e error) {
	if e == nil {
		return
	}

	message := e.Error() + " (" + r.RequestURI + " " + r.RemoteAddr + ")"
	log.Println(message)
}

type Server struct {
	lastConnId int64

	Addr        string
	SessionName string
	SessionTtl  int

	Hub      *hub.Hub
	Proto    *simple.Proto
	Sessions *ramSession.Registry
	Users    *ramUser.Registry
	Http     *http.Server

	mux        *http.ServeMux
	oldHandler http.Handler

	waitGroup     sync.WaitGroup
	refreshQueues int
	refreshPeriod time.Duration
	refreshChans  []chan refreshItem

	fs *fserv.Factory

	starting, running, stopping bool
}

func New() *Server {
	result := &Server{
		Addr:          DefaultAddr,
		SessionName:   DefaultSessionName,
		SessionTtl:    DefaultSessionTtl,
		refreshQueues: RefreshQueues,
		refreshPeriod: RefreshPeriod,
		mux:           http.NewServeMux(),
		fs:            fserv.NewFactory(),
	}

	return result
}

func (s *Server) AddFilePath(urlPath, fsPath string) {
	s.mux.Handle(urlPath, s.fs.Make(urlPath, fsPath))
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	handler, path := s.mux.Handler(r)
	if path != "" {
		handler.ServeHTTP(w, r)
	} else {
		s.oldHandler.ServeHTTP(w, r)
	}
}

func (s *Server) sessionCookie(sess *ramSession.Session) *http.Cookie {
	return &http.Cookie{Name: s.SessionName, Value: sess.Id(), MaxAge: s.SessionTtl}
}

func deleteCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, MaxAge: -1})
}

func (s *Server) whoami(w http.ResponseWriter, r *http.Request) (sess *ramSession.Session, user interface{}) {
	cookie, _ := r.Cookie(s.SessionName)
	if cookie == nil {
		return
	}

	sess = s.Sessions.Session(cookie.Value)
	if sess == nil {
		deleteCookie(w, s.SessionName)
		return
	}

	sess.Touch()

	u, ack := s.Users.User(sess.UserId())
	if !ack {
		deleteCookie(w, s.SessionName)
		s.Sessions.Delete(sess.Id())
		sess = nil
		return
	}

	http.SetCookie(w, s.sessionCookie(sess))
	user = u
	return
}

func serveJson(w http.ResponseWriter, r *http.Request, data interface{}) {
	response, e := json.Marshal(data)
	logRequest(r, e)
	w.Header().Set("Content-Type", "text/json")
	_, e = w.Write(response)
	logRequest(r, e)
}

func (s *Server) serveWhoami(w http.ResponseWriter, r *http.Request) {
	_, user := s.whoami(w, r)
	serveJson(w, r, whoamiRec{true, user})
}

func (s *Server) serveLogin(w http.ResponseWriter, r *http.Request) {
	sess, user := s.whoami(w, r)
	if user != nil {
		serveJson(w, r, whoamiRec{false, user})
		return
	}

	name := strings.TrimSpace(r.PostFormValue("name"))
	uid := s.Users.UserIdByName(name)
	if uid == 0 {
		uid = s.Users.AddUser(name)
	}
	sess = s.Sessions.NewSession(uid)
	http.SetCookie(w, s.sessionCookie(sess))
	serveJson(w, r, whoamiRec{true, &ramUser.UserEntry{uid, name}})
}

func (s *Server) serveLogout(w http.ResponseWriter, r *http.Request) {
	sess, user := s.whoami(w, r)
	if user != nil {
		s.Sessions.Delete(sess.Id())
		deleteCookie(w, s.SessionName)
	}

	serveJson(w, r, whoamiRec{true, nil})
}

func (s *Server) serveWs(w http.ResponseWriter, r *http.Request) {
	if !s.running || s.stopping {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	sess, user := s.whoami(w, r)
	if user == nil {
		logRequest(r, errors.New("anon"))
		w.WriteHeader(http.StatusForbidden)
		return
	}

	id := s.newId()
	conn, e := ws.New(w, r, s.Proto, int(id), sess.UserId())
	if e != nil {
		s.reuseId(id)
		logRequest(r, e)
		return
	}

	s.Proto.Connect(conn)
	s.refreshChans[int(id)%s.refreshQueues] <- refreshItem{conn, sess}
}

func (s *Server) newId() int64 {
	return atomic.AddInt64(&s.lastConnId, 1)
}

func (s *Server) reuseId(id int64) {
	atomic.CompareAndSwapInt64(&s.lastConnId, id, id-1)
}

func (s *Server) init() {
	s.mux.HandleFunc(WsPath, s.serveWs)
	s.mux.HandleFunc(WhoamiPath, s.serveWhoami)
	s.mux.HandleFunc(LoginPath, s.serveLogin)
	s.mux.HandleFunc(LogoutPath, s.serveLogout)

	if s.Users == nil {
		s.Users = ramUser.NewRegistry()
	}
	if s.Sessions == nil {
		s.Sessions = ramSession.NewRegistry()
	}
	if s.Hub == nil {
		s.Hub = hub.New(hub.NewMemStorage())
	}
	if s.Proto == nil {
		s.Proto = simple.New(s.Hub, s.Users, ramRoom.NewRegistry(), simpleAC.NewAccessController())
	}

	if s.Http != nil {
		s.oldHandler = s.Http.Handler
	} else {
		s.Http = &http.Server{Addr: s.Addr}
	}

	if s.oldHandler != nil {
		s.Http.Handler = http.HandlerFunc(s.serve)
	} else {
		s.Http.Handler = s.mux
	}
}

func (s *Server) start() {
	s.Hub.Start()
	s.waitGroup.Add(s.refreshQueues)
	s.refreshChans = make([]chan refreshItem, 0, s.refreshQueues)
	for i := 0; i < s.refreshQueues; i++ {
		ch := make(chan refreshItem, 4)
		s.refreshChans = append(s.refreshChans, ch)
		go s.goRefreshSessions(ch)
	}
}

func (s *Server) done() {
	for _, ch := range s.refreshChans {
		close(ch)
	}
	s.refreshChans = make([]chan refreshItem, 0)

	s.Hub.Stop()
	s.waitGroup.Wait()
}

func (s *Server) Run() error {
	if s.starting || s.running || s.stopping {
		return errors.New("chat server already running")
	}

	s.starting = true
	s.init()
	s.start()

	log.Println("listening")
	s.running = true
	e := s.Http.ListenAndServe()
	log.Println("cleanup")

	s.stopping = true
	s.done()
	return e
}

func (s *Server) Stop() {
	log.Println("stop request")
	if s.running {
		ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
		e := s.Http.Shutdown(ctx)
		if e != nil {
			log.Println(e)
		}
	}
}

func (s *Server) goRefreshSessions(queue <-chan refreshItem) {
	items := make([]*refreshItem, 0)
	timer := time.NewTicker(s.refreshPeriod)

Loop:
	for {
		select {
		case <-timer.C:
			l := len(items)
			i := 0
			for i < l {
				ip := items[i]
				if ip.conn.IsAlive() {
					ip.session.Touch()
					i++
				} else {
					l--
					items[i] = items[l]
					items[l] = nil
				}
			}
			if l < len(items) {
				items = items[:l]
			}

		case item, alive := <-queue:
			if !alive {
				break Loop
			}

			items = append(items, &item)
		}
	}

	timer.Stop()
	s.waitGroup.Done()
}

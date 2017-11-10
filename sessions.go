package sessions

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
)

const (
	DefaultKey  = "lemuse-session"
	errorFormat = "[sessions] ERROR! %s\n"
)

type Store interface {
	sessions.Store
	Options(Options)
}

// Options stores configuration for a session or session store.
// Fields are a subset of http.Cookie fields.
type Options struct {
	Path   string
	Domain string
	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'.
	// MaxAge>0 means Max-Age attribute present and given in seconds.
	MaxAge   int
	Secure   bool
	HttpOnly bool
}

type Session struct {
	name         string
	request      *http.Request
	store        Store
	session      *sessions.Session
	valueChanged bool //flag to mark if value of session changed
	writer       http.ResponseWriter
}

func (s *Session) Get(key interface{}) interface{} {
	return s.GetSession().Values[key]
}

func (s *Session) Set(key interface{}, val interface{}) {

	s.GetSession().Values[key] = val
	s.valueChanged = true
	//save data if value changed
	s.Save()
}

func (s *Session) Delete(key interface{}) {

	delete(s.GetSession().Values, key)
	s.valueChanged = true
}

func (s *Session) Clear() {
	for key := range s.GetSession().Values {
		s.Delete(key)
	}
}

func (s *Session) AddFlash(value interface{}, vars ...string) {
	s.GetSession().AddFlash(value, vars...)
	s.valueChanged = true
}

func (s *Session) Flashes(vars ...string) []interface{} {
	s.valueChanged = true
	return s.GetSession().Flashes(vars...)
}

func (s *Session) Options(options Options) {
	s.GetSession().Options = &sessions.Options{
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HttpOnly,
	}
}

func (s *Session) Save() error {
	if s.valueChanged {
		//check if session changed
		e := s.GetSession().Save(s.request, s.writer)
		if e == nil {
			s.valueChanged = false
		}
		return e
	}
	return nil
}

// Session returns a session with a specified name
func (s *Session) GetSession() *sessions.Session {
	if s.session == nil {
		var err error
		s.session, err = s.store.Get(s.request, s.name)
		if err != nil {
			log.Printf(errorFormat, err)
		}
	}
	return s.session
}

// SessionMiddware is a middware function only available for gin framework
func SessionMiddware(name string, store Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		s := &Session{name, c.Request, store, nil, false, c.Writer}
		c.Set(DefaultKey, s)
		defer context.Clear(c.Request)
		c.Next()
	}
}

// shortcut to get session
func Default(c *gin.Context) Session {
	return *(c.MustGet(DefaultKey).(*Session))
}

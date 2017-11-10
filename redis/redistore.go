// Copyright 2012 Brian "bojo" Jones. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package redistore

import (
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/go-redis/redis"
)

// Amount of time for cookies/redis keys to expire.
var sessionExpire = 86400 * 30

// RediStore stores sessions in a redis backend.
type RediStore struct {
	Client        *redis.Client
	ClusterClient *redis.ClusterClient
	Codecs        []securecookie.Codec
	Options       *sessions.Options // default configuration
	DefaultMaxAge int               // default Redis TTL for a MaxAge == 0 session
	maxLength     int
	keyPrefix     string
	IsCluster     bool
}

// SetMaxLength sets RediStore.maxLength if the `l` argument is greater or equal 0
// maxLength restricts the maximum length of new sessions to l.
// If l is 0 there is no limit to the size of a session, use with caution.
// The default for a new RediStore is 4096. Redis allows for max.
// value sizes of up to 512MB (http://redis.io/topics/data-types)
// Default: 4096,
func (s *RediStore) SetMaxLength(l int) {
	if l >= 0 {
		s.maxLength = l
	}
}

// SetKeyPrefix set the prefix
func (s *RediStore) SetKeyPrefix(p string) {
	s.keyPrefix = p
}

// SetMaxAge restricts the maximum age, in seconds, of the session record
// both in database and a browser. This is to change session storage configuration.
// If you want just to remove session use your session `s` object and change it's
// `Options.MaxAge` to -1, as specified in
//    http://godoc.org/github.com/gorilla/sessions#Options
//
// Default is the one provided by this package value - `sessionExpire`.
// Set it to 0 for no restriction.
// Because we use `MaxAge` also in SecureCookie crypting algorithm you should
// use this function to change `MaxAge` value.
func (s *RediStore) SetMaxAge(v int) {
	var c *securecookie.SecureCookie
	var ok bool
	s.Options.MaxAge = v
	for i := range s.Codecs {
		if c, ok = s.Codecs[i].(*securecookie.SecureCookie); ok {
			c.MaxAge(v)
		} else {
			fmt.Printf("Can't change MaxAge on codec %v\n", s.Codecs[i])
		}
	}
}

// NewRediStore returns a new RediStore.
// size: maximum number of idle connections.
func NewRediStore(isCluster bool, size int, address []string, password string, keyPairs ...[]byte) (*RediStore, error) {
	var rs *RediStore
	if isCluster {
		if len(address) < 6 {
			panic("cluster mode. redis cluster address error. count < 6")
		}
		client := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:       address,
			PoolSize:    size,
			DialTimeout: 10 * time.Second,
			Password:    password,
		})

		rs = &RediStore{
			ClusterClient: client,
			Codecs:        securecookie.CodecsFromPairs(keyPairs...),
			Options: &sessions.Options{
				Path:   "/",
				MaxAge: sessionExpire,
			},
			DefaultMaxAge: 60 * 30, // 30 minutes seems like a reasonable default
			maxLength:     4096,
			keyPrefix:     "session_",
			IsCluster:     true,
		}
	} else {
		if len(address) > 1 {
			panic("single mode. redis address error. count > 1")
		}
		client := redis.NewClient(&redis.Options{
			Addr:        address[0],
			PoolSize:    size,
			DialTimeout: 10 * time.Second,
			Password:    password,
		})
		rs = &RediStore{
			Client: client,
			Codecs: securecookie.CodecsFromPairs(keyPairs...),
			Options: &sessions.Options{
				Path:   "/",
				MaxAge: sessionExpire,
			},
			DefaultMaxAge: 60 * 30, // 30 minutes seems like a reasonable default
			maxLength:     4096,
			keyPrefix:     "session_",
			IsCluster:     false,
		}
	}

	_, err := rs.ping()
	return rs, err
}

// Close closes the underlying *redis.Pool
func (s *RediStore) Close() error {
	return s.Client.Close()
}

// Get returns a session for the given name after adding it to the registry.
//
// See gorilla/sessions FilesystemStore.Get().
func (s *RediStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New returns a session for the given name without adding it to the registry.
//
// See gorilla/sessions FilesystemStore.New().
func (s *RediStore) New(r *http.Request, name string) (*sessions.Session, error) {
	var err error
	session := sessions.NewSession(s, name)
	// make a copy
	options := *s.Options
	session.Options = &options
	session.IsNew = true
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, c.Value, &session.ID, s.Codecs...)
		if err == nil {
			err = s.load(session)
			session.IsNew = !(err == nil) // not new if no error and data available
		}
	}
	return session, err
}

// Save adds a single session to the response.
func (s *RediStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	// Marked for deletion.
	if session.Options.MaxAge < 0 {
		if err := s.delete(session); err != nil {
			return err
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
	} else {
		// Build an alphanumeric key for the redis store.
		// generate the session id
		if session.ID == "" {
			session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
		}
		if err := s.save(session); err != nil {
			return err
		}
		encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.Codecs...)
		if err != nil {
			return err
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	}
	return nil
}

// ping does an internal ping against a server to check if it is alive.
func (s *RediStore) ping() (bool, error) {
	var data string
	var err error

	if s.IsCluster {
		data, err = s.ClusterClient.Ping().Result()
	} else {
		data, err = s.Client.Ping().Result()
	}

	if err != nil || data == "" {
		return false, err
	}
	return (data == "PONG"), nil
}

// save stores the session in redis.
func (s *RediStore) save(session *sessions.Session) error {
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values,
		s.Codecs...)
	if err != nil {
		return err
	}
	if s.maxLength != 0 && len(encoded) > s.maxLength {
		return errors.New("SessionStore: the value to store is too big")
	}

	age := session.Options.MaxAge
	if age == 0 {
		age = s.DefaultMaxAge
	}
	// encode with secure cookie
	if s.IsCluster {
		_, err = s.ClusterClient.Set(s.keyPrefix+session.ID, encoded, time.Duration(age)*time.Second).Result()
	} else {
		_, err = s.Client.Set(s.keyPrefix+session.ID, encoded, time.Duration(age)*time.Second).Result()
	}
	return err
}

// load reads the session from redis.
// returns true if there is a sessoin data in DB
func (s *RediStore) load(session *sessions.Session) error {
	var err error
	var data string
	if s.IsCluster {
		data, err = s.ClusterClient.Get(s.keyPrefix + session.ID).Result()
	} else {
		data, err = s.Client.Get(s.keyPrefix + session.ID).Result()
	}
	if data == "" {
		return nil // no data was associated with this key
	}
	// decode
	if err = securecookie.DecodeMulti(session.Name(), data,
		&session.Values, s.Codecs...); err != nil {
		return err
	}
	return nil
}

// delete removes keys from redis if MaxAge<0
func (s *RediStore) delete(session *sessions.Session) error {
	var err error
	if s.IsCluster {
		_, err = s.ClusterClient.Del(s.keyPrefix + session.ID).Result()
	} else {
		_, err = s.Client.Del(s.keyPrefix + session.ID).Result()
	}

	return err
}

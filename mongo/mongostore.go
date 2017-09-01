// Copyright 2012 The KidStuff Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mongostore

import (
	"errors"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"net/http"
	"time"
)

var (
	ErrInvalidId = errors.New("mgostore: invalid session id")
)

const (
	SessionDbName = "test"
	SessionCollectionName = "session_test"
)

// Session object store in MongoDB
type SessionItem struct {
	Id       bson.ObjectId `bson:"_id,omitempty"`
	Data     string `bson:"data"`
	Modified time.Time `bson:"modified"`
}


// MongoStore stores sessions in MongoDB
type MongoStore struct {
	Codecs           []securecookie.Codec
	Options          *sessions.Options
	GlobalMgoSession *mgo.Session
}

// NewMongoStore returns a new MongoStore.
// Set ensureTTL to true let the database auto-remove expired object by maxAge.
func NewMongoStore(globalMgoSession *mgo.Session, maxAge int, ensureTTL bool,
keyPairs ...[]byte) *MongoStore {
	store := &MongoStore{
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		Options: &sessions.Options{
			Path: "/",
			MaxAge: maxAge,
		},
		GlobalMgoSession:  globalMgoSession,
	}
	globalMgoSession.SetMode(mgo.Monotonic, true)
	//default is 4096
	globalMgoSession.SetPoolLimit(1000)

	mgoSession := globalMgoSession.Clone()
	defer mgoSession.Close()
	c := mgoSession.DB(SessionDbName).C(SessionCollectionName)

	if ensureTTL {
		c.EnsureIndex(mgo.Index{
			Key:         []string{"modified"},
			Background:  true,
			Sparse:      true,
			ExpireAfter: time.Duration(maxAge) * time.Second,
		})
	}

	return store
}

// Get registers and returns a session for the given name and session store.
// It returns a new session if there are no sessions registered for the name.
func (m *MongoStore) Get(r *http.Request, name string) (
*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(m, name)
}

// New returns a session for the given name without adding it to the registry.
func (m *MongoStore) New(r *http.Request, name string) (
*sessions.Session, error) {
	session := sessions.NewSession(m, name)
	session.Options = &sessions.Options{
		Path:     m.Options.Path,
		MaxAge:   m.Options.MaxAge,
		Domain:   m.Options.Domain,
		Secure:   m.Options.Secure,
		HttpOnly: m.Options.HttpOnly,
	}
	session.IsNew = true
	var err error
	if cookie, err := r.Cookie(name); err == nil {
		cookieVal := cookie.Value
		err = securecookie.DecodeMulti(name, cookieVal, &session.ID, m.Codecs...)
		if err == nil {
			err = m.load(session)
			if err == nil {
				session.IsNew = false
			} else {
				err = nil
			}
		}
	}
	return session, err
}

// Save saves all sessions registered for the current request.
func (m *MongoStore) Save(r *http.Request, w http.ResponseWriter,
session *sessions.Session) error {
	if session.Options.MaxAge < 0 {
		if err := m.delete(session); err != nil {
			return err
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	if session.ID == "" {
		session.ID = bson.NewObjectId().Hex()
	}

	if err := m.upsert(session); err != nil {
		return err
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID,
		m.Codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

func (m *MongoStore) load(session *sessions.Session) error {
	if !bson.IsObjectIdHex(session.ID) {
		return ErrInvalidId
	}
	s := SessionItem{}

	mgoSession := m.GlobalMgoSession.Clone()
	defer mgoSession.Close()
	c := mgoSession.DB(SessionDbName).C(SessionCollectionName)

	err := c.FindId(bson.ObjectIdHex(session.ID)).One(&s)
	if err != nil {
		return err
	}

	if err := securecookie.DecodeMulti(session.Name(), s.Data, &session.Values,
		m.Codecs...); err != nil {
		return err
	}

	return nil
}

func (m *MongoStore) upsert(session *sessions.Session) error {
	if !bson.IsObjectIdHex(session.ID) {
		return ErrInvalidId
	}

	var modified time.Time
	if val, ok := session.Values["modified"]; ok {
		modified, ok = val.(time.Time)
		if !ok {
			return errors.New("mongostore: invalid modified value")
		}
	} else {
		modified = time.Now()
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values,
		m.Codecs...)
	if err != nil {
		return err
	}

	s := SessionItem{
		Id:       bson.ObjectIdHex(session.ID),
		Data:     encoded,
		Modified: modified,
	}

	mgoSession := m.GlobalMgoSession.Clone()
	defer mgoSession.Close()
	c := mgoSession.DB(SessionDbName).C(SessionCollectionName)

	_, err = c.UpsertId(s.Id, &s)
	if err != nil {
		return err
	}

	return nil
}

func (m *MongoStore) delete(session *sessions.Session) error {
	if !bson.IsObjectIdHex(session.ID) {
		return ErrInvalidId
	}

	mgoSession := m.GlobalMgoSession.Clone()
	defer mgoSession.Close()
	c := mgoSession.DB(SessionDbName).C(SessionCollectionName)

	return c.RemoveId(bson.ObjectIdHex(session.ID))
}

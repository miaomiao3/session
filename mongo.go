package sessions

import (
	"github.com/gorilla/sessions"
	mgo "gopkg.in/mgo.v2"
	"github.com/miaomiao3/session/mongo"
)

type MongoStore interface {
	Store
}

func NewMongoStore(s *mgo.Session, maxAge int, ensureTTL bool, keyPairs ...[]byte) MongoStore {
	store := mongostore.NewMongoStore(s, maxAge, ensureTTL, keyPairs...)

	return &mongoStore{store}
}

type mongoStore struct {
	*mongostore.MongoStore
}

func (c *mongoStore) Options(options Options) {
	c.MongoStore.Options = &sessions.Options{
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HttpOnly,
	}
}

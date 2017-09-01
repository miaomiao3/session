## session

Main idea is from:  https://github.com/gin-contrib/sessions

Compare with original:

* 1, codes are less than the original.
* 2, use go-redis as redis client(https://github.com/go-redis/redis) which supports redis-cluster


Examples

### Redis Single
``` 
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/miaomiao3/session"
)

func main() {
	r := gin.Default()

	store, _ := sessions.NewRedisStore(false, 100,[]string{":6379"},
		"",  []byte("hash-key21"),[]byte("block-key2110000"))

	r.Use(sessions.SessionMiddware("mm-session", store))
	store.Options(sessions.Options{
		Path:"/",
		//Domain:"localhost",
		HttpOnly:false,
		Secure:false,
		MaxAge:3600,
	})

	r.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		var count int
		v := session.Get("count")
		if v == nil {
			count = 0
		} else {
			count = v.(int)
			count++
		}
		session.Set("count", count)
		session.Save()
		c.JSON(200, gin.H{"count": count})
	})
	r.Run(":8092") // listen and serve on 0.0.0.0:8080
}

```
***

### Redis cluster
``` 
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/miaomiao3/session"
)

func main() {
	r := gin.Default()

	store, _ := sessions.NewRedisStore(true, 100, []string{":6380",":6381",":6382",":6383",":6384",":6385"},
    		"",  []byte("hash-key21"),[]byte("block-key2110000"))

	r.Use(sessions.SessionMiddware("mm-session", store))
	store.Options(sessions.Options{
		Path:"/",
		//Domain:"localhost",
		HttpOnly:false,
		Secure:false,
		MaxAge:3600,
	})

	r.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		var count int
		v := session.Get("count")
		if v == nil {
			count = 0
		} else {
			count = v.(int)
			count++
		}
		session.Set("count", count)
		session.Save()
		c.JSON(200, gin.H{"count": count})
	})
	r.Run(":8092") // listen and serve on 0.0.0.0:8080
}

```



### MongoDB
``` 
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/miaomiao3/session"
)

func main() {
	r := gin.Default()

	mgoSessieon, err := mgo.Dial("localhost:27017/test")
    if err != nil {
    		// handle err
    }
    
    store := sessions.NewMongoStore(mgoSessieon, 3600, true, []byte("secret-key"))

	r.Use(sessions.SessionMiddware("mm-session", store))
	store.Options(sessions.Options{
		Path:"/",
		//Domain:"localhost",
		HttpOnly:false,
		Secure:false,
		MaxAge:3600,
	})

	r.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		var count int
		v := session.Get("count")
		if v == nil {
			count = 0
		} else {
			count = v.(int)
			count++
		}
		session.Set("count", count)
		session.Save()
		c.JSON(200, gin.H{"count": count})
	})
	r.Run(":8092") // listen and serve on 0.0.0.0:8080
}

```
***




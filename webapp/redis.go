// dns-server-roleplay/webapp/redis.go
package main

import (
	"github.com/go-redis/redis/v8"
)

var rdb *redis.Client

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
}


// Copyright 2017 gistao, xiaofei
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package main

import (
	"github.com/gistao/RedisGo-Async/redis"
	"github.com/montanaflynn/stats"
	"log"
	"math/rand"
	"sync"
	"time"
)

type RedisClient struct {
	// pool *redis.Pool
	pool *redis.AsyncPool
	Addr string
}

var (
	cliMap map[string]*RedisClient
	mutex  *sync.RWMutex
)

func init() {
	cliMap = make(map[string]*RedisClient)
	mutex = new(sync.RWMutex)
}

func newSyncPool(addr string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     100,
		MaxActive:   100,
		IdleTimeout: time.Minute * 1,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr)
			return c, err
		},
	}
}

func newAsyncPool(addr string) *redis.AsyncPool {
	return &redis.AsyncPool{
		Dial: func() (redis.AsynConn, error) {
			c, err := redis.AsyncDial("tcp", addr)
			return c, err
		},
		MaxGetCount: 1000,
	}
}

func GetRedisClient(addr string) *RedisClient {
	var redis *RedisClient
	var ok bool
	mutex.RLock()
	redis, ok = cliMap[addr]
	mutex.RUnlock()
	if !ok {
		mutex.Lock()
		redis, ok = cliMap[addr]
		if !ok {
			//redis = &RedisClient{pool: newSyncPool(addr), Addr: addr}
			redis = &RedisClient{pool: newAsyncPool(addr), Addr: addr}
			cliMap[addr] = redis
		}
		mutex.Unlock()
	}
	return redis
}

func (c *RedisClient) Exists(key string) (int64, error) {
	conn := c.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("EXISTS", key)
	if err == nil && reply == nil {
		return 0, nil
	}
	val, err := redis.Int64(reply, err)
	return val, err
}

func (c *RedisClient) Get(key string) (string, error) {
	conn := c.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("GET", key)
	if err == nil && reply == nil {
		return "", nil
	}
	val, err := redis.String(reply, err)
	return val, err
}

func (c *RedisClient) Del(key string) (int64, error) {
	conn := c.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("DEL", key)
	if err == nil && reply == nil {
		return 0, nil
	}
	val, err := redis.Int64(reply, err)
	return val, err
}

func (c *RedisClient) HGet(hashID string, field string) (string, error) {
	conn := c.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("HGET", hashID, field)
	if err == nil && reply == nil {
		return "", nil
	}
	val, err := redis.String(reply, err)
	return val, err
}

func (c *RedisClient) INCR(key string) (int, error) {
	conn := c.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("INCR", key)
	if err == nil && reply == nil {
		return 0, nil
	}
	val, err := redis.Int(reply, err)
	return val, err
}

func (c *RedisClient) DECR(key string) (int, error) {
	conn := c.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("DECR", key)
	if err == nil && reply == nil {
		return 0, nil
	}
	val, err := redis.Int(reply, err)
	return val, err
}

func (c *RedisClient) HGetAll(hashID string) (map[string]string, error) {
	conn := c.pool.Get()
	defer conn.Close()
	reply, err := redis.StringMap(conn.Do("HGetAll", hashID))
	return reply, err
}

func (c *RedisClient) HSet(hashID string, field string, val string) error {
	conn := c.pool.Get()
	defer conn.Close()
	_, err := conn.Do("HSET", hashID, field, val)
	return err
}

func (c *RedisClient) HMSet(args ...interface{}) error {
	conn := c.pool.Get()
	defer conn.Close()
	_, err := conn.Do("HMSET", args...)
	return err
}

func (c *RedisClient) Expire(key string, expire int64) error {
	conn := c.pool.Get()
	defer conn.Close()
	_, err := conn.Do("EXPIRE", key, expire)
	return err
}

func (c *RedisClient) Set(key string, val string) (string, error) {
	conn := c.pool.Get()
	defer conn.Close()
	val, err := redis.String(conn.Do("SET", key, val))
	return val, err
}

func (c *RedisClient) SetWithExpire(key string, val string, timeOutSeconds int) (string, error) {
	conn := c.pool.Get()
	defer conn.Close()
	val, err := redis.String(conn.Do("SET", key, val, "EX", timeOutSeconds))
	return val, err
}

func (c *RedisClient) GetTTL(key string) (int64, error) {
	conn := c.pool.Get()
	defer conn.Close()
	val, err := redis.Int64(conn.Do("TTL", key))
	return val, err
}

func (c *RedisClient) ZAdd(args ...interface{}) error {
	conn := c.pool.Get()
	defer conn.Close()
	_, err := conn.Do("ZADD", args...)
	return err
}

// list操作
func (c *RedisClient) LLen(key string) (int64, error) {
	conn := c.pool.Get()
	defer conn.Close()
	val, err := redis.Int64(conn.Do("LLEN", key))
	return val, err
}

func (c *RedisClient) RPopLPush(src, dst string) (string, error) {
	conn := c.pool.Get()
	defer conn.Close()
	val, err := redis.String(conn.Do("RPOPLPUSH", src, dst))
	return val, err
}

func (c *RedisClient) RPop(key string) (string, error) {
	conn := c.pool.Get()
	defer conn.Close()
	val, err := redis.String(conn.Do("RPOP", key))
	return val, err
}

func (c *RedisClient) LPop(key string) (string, error) {
	conn := c.pool.Get()
	defer conn.Close()
	val, err := redis.String(conn.Do("LPOP", key))
	return val, err
}

func (c *RedisClient) RPush(key string, val string) (int64, error) {
	conn := c.pool.Get()
	defer conn.Close()
	ret, err := redis.Int64(conn.Do("RPUSH", key, val))
	if err != nil {
		return -1, err
	} else {
		return ret, nil
	}
}

func (c *RedisClient) LPush(key string, val string) (int64, error) {
	conn := c.pool.Get()
	defer conn.Close()
	ret, err := redis.Int64(conn.Do("LPUSH", key, val))
	if err != nil {
		return -1, err
	} else {
		return ret, nil
	}
}

func (c *RedisClient) MkSet(pairs chan struct {k string; v []byte}) ([]byte, error) {
	conn := c.pool.Get()
	defer conn.Close()
	var ret redis.AsyncRet
	var err error
	for pair := range pairs{
		log.Println(pairs)
		ret, err = conn.AsyncDo("SET", pair.k, pair.v)
		if err != nil {
			log.Println("Err: ", err)
		}
	}
	v, err := ret.Get()
	b, err := redis.Bytes(v, err)
	if err != nil {
		log.Println("Err: ", err)
	}else{
		log.Println("success: ", v)
	}
	return b, err

}


func main() {
	log.SetPrefix("[RedisGo-Async|example] ")
	// get client
	rdc := GetRedisClient("10.4.5.92:6379")
	key := "hello-redisgo-async"
	val := "hello world"
	_, err := rdc.Set(key, val)
	if err != nil {
		log.Println(err)
	}else{
		log.Printf("set %s value \"%s\" success\n", key, val)
	}

	ret, err := rdc.Get(key)
	if err != nil {
		log.Println(err)
	}else{
		log.Printf("get %s result \"%s\"\n", key, ret)
	}

	_, err = rdc.Del(key)
	if err != nil {
		log.Println(err)
	}else{
		log.Printf("del key %s success\n", key)

	}

	size := 160
	v := make([]byte, size)
	rand.Read(v)
	pairs := make(chan struct {k string; v []byte}, 3)
	pairs <- struct {k string; v []byte}{"k1", v}
	pairs <- struct {k string; v []byte}{"k2", v}
	pairs <- struct {k string; v []byte}{"k3", v}
	pairs <- struct {k string; v []byte}{"k4", v}
	pairs <- struct {k string; v []byte}{"k5", v}
	pairs <- struct {k string; v []byte}{"k6", v}

	close(pairs)
	_, err = rdc.MkSet(pairs)
	if err != nil{
		log.Println("Err: ", err)
	}

	var statistics []float64
	for i:=0;i<1000;i++ {
		log.Println(i)
		start := time.Now()
		rdc.MkSet(pairs)
		t := time.Since(start)
		statistics = append(statistics, t.Seconds()*1e3)
	}

	log.Println(stats.Mean(statistics))
}
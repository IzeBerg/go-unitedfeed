package main

import (
	"github.com/getsentry/sentry-go"
	"github.com/go-redis/redis"
	"io/ioutil"
	"os"
	"path/filepath"
)

func initFileDatabase(db *redis.Client, key, path string) {
	if data, err := db.Get(key).Bytes(); err == nil {
		os.MkdirAll(path, os.ModePerm)
		if err := ioutil.WriteFile(filepath.Join(path, `td.binlog`), data, os.ModePerm); err != nil {
			sentry.CaptureException(err)
			panic(err)
		}
	} else if err != redis.Nil {
		sentry.CaptureException(err)
		panic(err)
	}
}

func saveFileDatabase(db *redis.Client, key, path string) {
	if data, err := ioutil.ReadFile(filepath.Join(path, `td.binlog`)); err == nil {
		if err := db.Set(key, data, 0).Err(); err != nil {
			sentry.CaptureException(err)
			panic(err)
		}
	} else {
		sentry.CaptureException(err)
		panic(err)
	}
}

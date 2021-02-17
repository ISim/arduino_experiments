package firestore

import (
	"context"
	"log"
	firebase "firebase.google.com/go"
	"os"
	"time"
)

var app *firebase.App

func init() {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	conf := &firebase.Config{ProjectID: os.Getenv("GOOGLE_CLOUD_PROJECT")}
	app, err = firebase.NewApp(ctx, conf)
	if err != nil {
		log.Printf("can't connect ro firebase: %s", err.Error())
	}
}

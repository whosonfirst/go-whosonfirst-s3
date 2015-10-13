package main

import (
	"flag"
	"fmt"
	"github.com/goamz/goamz/aws"
	"github.com/whosonfirst/go-whosonfirst-s3"
	"os"
)

func main() {

	var root = flag.String("root", "", "The directory to sync")
	var bucket = flag.String("bucket", "", "The S3 bucket to sync <root> to")
	var prefix = flag.String("prefix", "", "A prefix inside your S3 bucket where things go")
	var debug = flag.Bool("debug", false, "Don't actually try to sync anything and spew a lot of line noise")
	var credentials = flag.String("credentials", "", "Your S3 credentials file")

	flag.Parse()

	if *root == "" {
		panic("missing root to sync")
	}

	_, err := os.Stat(*root)

	if os.IsNotExist(err) {
		panic("root does not exist")
	}

	if *bucket == "" {
		panic("missing bucket")
	}

	if *credentials != "" {
		os.Setenv("AWS_CREDENTIAL_FILE", *credentials)
	}

	auth, err := aws.SharedAuth()

	if err != nil {
		panic(err)
	}

	// sudo figure out how to put all of the log
	// channel stuff into the Sync object itself
	// (20150930/thisisaaronland)

	log := make(chan string)

	cb := func(cs chan string) {
		s := <-cs
		fmt.Println(s)
	}

	go cb(log)

	s := s3.WOFSync(auth, *bucket, *prefix, log)
	err = s.SyncDirectory(*root, *debug)

	close(log)
}

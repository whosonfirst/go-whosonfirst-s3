prep:
	if test -d pkg; then rm -rf pkg; fi

self:	prep
	if test -d src/github.com/whosonfirst/go-whosonfirst-s3; then rm -rf src/github.com/whosonfirst/go-whosonfirst-s3; fi
	mkdir -p src/github.com/whosonfirst/go-whosonfirst-s3
	cp s3.go src/github.com/whosonfirst/go-whosonfirst-s3/

deps: 	self
	go get -u "github.com/whosonfirst/go-whosonfirst-crawl"
	go get -u "github.com/goamz/goamz/aws"
	go get -u "github.com/goamz/goamz/s3"
	go get -u "github.com/jeffail/tunny"

sync: 	self
	go build -o bin/wof-sync bin/wof-sync.go

fmt:
	go fmt *.go 
	go fmt bin/*.go

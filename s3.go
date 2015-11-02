package s3

// https://github.com/aws/aws-sdk-go
// https://docs.aws.amazon.com/sdk-for-go/api/service/s3.html

// https://github.com/goamz/goamz/blob/master/aws/aws.go
// https://github.com/goamz/goamz/blob/master/s3/s3.go

import (
	"crypto/md5"
	enc "encoding/hex"
	"github.com/goamz/goamz/aws"
	aws_s3 "github.com/goamz/goamz/s3"
	"github.com/jeffail/tunny"
	"github.com/whosonfirst/go-whosonfirst-crawl"
	log "github.com/whosonfirst/go-whosonfirst-log"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

type Sync struct {
	ACL    aws_s3.ACL
	Bucket aws_s3.Bucket
	Prefix string
	Pool   tunny.WorkPool
	Logger *log.WOFLogger
}

func NewSync(auth aws.Auth, region aws.Region, acl aws_s3.ACL, bucket string, prefix string, procs int, logger *log.WOFLogger) *Sync {

	runtime.GOMAXPROCS(procs)

	pool, _ := tunny.CreatePoolGeneric(procs).Open()

	s := aws_s3.New(auth, region)
	b := s.Bucket(bucket)

	return &Sync{
		ACL:    acl,
		Bucket: *b,
		Prefix: prefix,
		Pool:   *pool,
		Logger: logger,
	}
}

func WOFSync(auth aws.Auth, bucket string, prefix string, procs int, logger *log.WOFLogger) *Sync {

	return NewSync(auth, aws.USEast, aws_s3.PublicRead, bucket, prefix, procs, logger)
}

func (sink Sync) SyncDirectory(root string, debug bool) error {

	defer sink.Pool.Close()

	var files int64
	var failed int64

	t0 := time.Now()

	callback := func(src string, info os.FileInfo) error {

		sink.Logger.Debug("crawling %s", src)

		if info.IsDir() {
			return nil
		}

		files++

		source := src
		dest := source

		dest = strings.Replace(dest, root, "", -1)

		if sink.Prefix != "" {
			dest = path.Join(sink.Prefix, dest)
		}

		// Note: both HasChanged and SyncFile will ioutil.ReadFile(source)
		// which is a potential waste of time and resource. Or maybe we just
		// don't care? (20150930/thisisaaronland)

		sink.Logger.Debug("LOOKING FOR %s (%s)", dest, sink.Prefix)

		change, ch_err := sink.HasChanged(source, dest)

		if ch_err != nil {
			sink.Logger.Warning("failed to determine whether %s had changed, because '%s'", source, ch_err)
			change = true
		}

		if debug == true {
			sink.Logger.Debug("has %s changed? the answer is %v but does it really matter since debugging is enabled?", source, change)
		} else if change {

			s_err := sink.SyncFile(source, dest)

			if s_err != nil {
				sink.Logger.Error("failed to PUT %s, because '%s'", dest, s_err)
				failed++
			}
		} else {
			// pass
		}

		return nil
	}

	c := crawl.NewCrawler(root)
	_ = c.Crawl(callback)

	t1 := float64(time.Since(t0)) / 1e9

	sink.Logger.Info("processed %d files (error: %d) in %.3f seconds\n", files, failed, t1)

	return nil
}

func (sink Sync) SyncFile(source string, dest string) error {

	// sink.LogMessage(fmt.Sprintf("sync file %s", source))

	body, err := ioutil.ReadFile(source)

	if err != nil {
		sink.Logger.Error("Failed to read %s, because %v", source, err)
		return err
	}

	_, err = sink.Pool.SendWork(func() {

		sink.Logger.Debug("PUT %s as %s", dest, sink.ACL)

		o := aws_s3.Options{}

		err := sink.Bucket.Put(dest, body, "text/plain", sink.ACL, o)

		if err != nil {
			sink.Logger.Error("failed to PUT %s, because '%s'", dest, err)
		}

	})

	if err != nil {
		sink.Logger.Error("failed to schedule %s for processing, because '%s'", source, err)
		return err
	}

	// sink.Logger.Debug("scheduled %s for processing", source)
	return nil
}

// the following appears to trigger a freak-out-and-die condition... sometimes
// I have no idea why... test under go 1.2.1, 1.4.3 and 1.5.1 / see also:
// https://github.com/whosonfirst/go-mapzen-whosonfirst-s3/issues/2
// (2015/thisisaaronland)

func (sink Sync) HasChanged(source string, dest string) (ch bool, err error) {

	change := true

	body, err := ioutil.ReadFile(source)

	if err != nil {
		return change, err
	}

	hash := md5.Sum(body)
	local_hash := enc.EncodeToString(hash[:])

	headers := make(http.Header)
	rsp, err := sink.Bucket.Head(dest, headers)

	if err != nil {
		sink.Logger.Error("failed to HEAD %s because %s", dest, err)
		return change, err
	}

	etag := rsp.Header.Get("Etag")
	remote_hash := strings.Replace(etag, "\"", "", -1)

	if local_hash == remote_hash {
		change = false
	}

	return change, nil
}
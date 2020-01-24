package jpegcc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

// ErrMediaIsEmpty is returned when size of downloaded file is equal to zero.
var ErrMediaIsEmpty = errors.New("url referes to the empty file")

// MediaDownloader implements interface Downloader. Supports limiting connection per host,
// configurable amount of workers, uses fasthttp.Client to reduce garbage generation.
type MediaDownloader struct {
	log    zerolog.Logger
	input  Inputer
	imgs   chan Imager
	client fasthttp.Client
	wg     sync.WaitGroup
}

// Media implement interface Imager and represents downloaded image stored in the memory.
type Media struct {
	resp *fasthttp.Response
	req  *fasthttp.Request
	url  string
}

const (
	// DefaultMaxConnsPerHost defines default value of maximum parallel http connections
	// to the host. To prevent DDoS.
	DefaultMaxConnsPerHost = 32

	// DefaultReadTimeout defines maximum duration for full response reading (including body).
	DefaultReadTimeout = 8 * time.Second
)

// NewMediaDownloader returns new instance of MediaDownloader, with default read timeout and
// MaxConnsPerHost (32) parameters.
func NewMediaDownloader(l zerolog.Logger, in Inputer) *MediaDownloader {
	return &MediaDownloader{
		log:   l.With().Str("component", "downloader").Logger(),
		input: in,
		imgs:  make(chan Imager, 10),
		client: fasthttp.Client{ReadTimeout: DefaultReadTimeout,
			MaxConnsPerHost:     1,
			ReadBufferSize:      6 * 1024 * 1024,
			MaxResponseBodySize: 16 * 1024 * 1024},
	}
}

// SetMaxConnsPerHost set maximum parallel http connections to the host.
func (id *MediaDownloader) SetMaxConnsPerHost(n int) {
	id.client.MaxConnsPerHost = n
}

// SetReadTimeout set maximum duration for full response reading (including body).
func (id *MediaDownloader) SetReadTimeout(d time.Duration) {
	id.client.ReadTimeout = d
}

// Start launches n parallel image download go-routines.
func (id *MediaDownloader) Start(ctx context.Context, n int) {
	go func() {
		for i := 0; i < n; i++ {
			id.wg.Add(1)
			go id.runner(ctx, i+1)
		}
		id.log.Debug().Int("amount", n).Msg("runners are started")
		id.wg.Wait()
		close(id.imgs)
	}()
}

func (id *MediaDownloader) runner(ctx context.Context, num int) {

	log := id.log.With().Int("runner", num).Logger()
	for {
		select {
		case <-ctx.Done():
			id.wg.Done()
			return
		case url, ok := <-id.input.Next():
			if !ok {
				// input channel is closed due to reaching end of file. Stop the runner.
				id.wg.Done()
				return
			}

			t := time.Now()
			img, err := id.Download(ctx, url)
			if err != nil {
				log.Error().Str("url", url).Str("errmsg", err.Error()).Msg("image download failed")
				break
			}

			size := len(img.Bytes())

			// catching ctx.Done() while imgs chan is full.
			select {
			case <-ctx.Done():
				id.wg.Done()
				return
			case id.imgs <- img:
				log.Debug().Str("url", url).Int("size", size).Str("dur", time.Since(t).String()).Msg("downloaded")
				break
			}
		}
	}
}

// Download retrive image by URL.
func (id *MediaDownloader) Download(ctx context.Context, url string) (Imager, error) {

	var err error

	img := Media{url: url,
		req:  fasthttp.AcquireRequest(),
		resp: fasthttp.AcquireResponse()}

	img.req.SetRequestURI(url)

	if err = fasthttp.Do(img.req, img.resp); err == nil {
		if code := img.resp.StatusCode(); code != 200 {
			img.Reset()
			return nil, fmt.Errorf("http code %d", code)
		}
		if len(img.resp.Body()) == 0 {
			img.Reset()
			return nil, ErrMediaIsEmpty
		}
		return &img, nil
	}

	if err != fasthttp.ErrNoFreeConns {
		img.Reset()
		return nil, err
	}

	// can be replaced with dymanically calculated delay in accordance
	// to average ratio (image size/download duration) for every host.
	ticker := time.NewTicker(25 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if err = fasthttp.Do(img.req, img.resp); err == nil {
				ticker.Stop()
				return &img, nil
			}
			if err != fasthttp.ErrNoFreeConns {
				ticker.Stop()
				return nil, err
			}
			break
		}
	}

}

// Next returns chan with downloaded Imagers ready to process.
func (id *MediaDownloader) Next() <-chan Imager {
	return id.imgs
}

// Reset implements interface Imager. Releases HTTP Body buffer.
func (i *Media) Reset() {
	i.resp.ResetBody()
	fasthttp.ReleaseResponse(i.resp)
	fasthttp.ReleaseRequest(i.req)
}

// Bytes returns image as []byte.
func (i *Media) Bytes() []byte {
	return i.resp.Body()
}

// URL returns URL of downloaded image.
func (i *Media) URL() string {
	return i.url
}

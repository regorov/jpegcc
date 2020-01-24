package jpegcc

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ImageProcessor implements core logic orchestration functionality.
// It reads Imager(s) from Downloader, invocates Counter for processing and uses Outputer to save result.
// It's suggested to limit max amount of parallel processing goroutins equal to amount of cores.
type ImageProcessor struct {
	down    Downloader
	output  Outputer
	counter Counter
	log     zerolog.Logger
	wg      sync.WaitGroup
}

// NewImageProcessor returns new instance of ImageProcessor.
func NewImageProcessor(l zerolog.Logger, d Downloader, o Outputer, c Counter) *ImageProcessor {
	return &ImageProcessor{
		log:     l.With().Str("component", "imgproc").Logger(),
		down:    d,
		output:  o,
		counter: c}
}

// Start launches n paraller processing goroutines and waits completion.
func (ip *ImageProcessor) Start(ctx context.Context, n int) {

	for i := 0; i < n; i++ {
		ip.wg.Add(1)
		go ip.runner(ctx, i+1)
	}
	ip.log.Debug().Int("amount", n).Msg("runners are started")
	ip.wg.Wait()
}

func (ip *ImageProcessor) runner(ctx context.Context, num int) {

	var (
		totalb  int // total amount of bytes passsed through the runner.
		msize   int // max image size in bytes passed passsed through the runner.
		cnt     int // amount if images passed through the runner.
		per100b int
	)
	started := time.Now()
	started100 := started
	log := ip.log.With().Int("runner", num).Logger()

	logstat := func(msg string) {
		log.Info().Int("count", cnt).
			Int("total-bytes", totalb).
			Int("total-bsec-thr", totalb/(int(time.Since(started).Seconds()+1))).
			Int("100-bsec-thr", per100b/(int(time.Since(started100).Seconds()+1))).
			Int("max-image-size", msize).Msg(msg)
	}

	for {
		select {
		case <-ctx.Done():
			logstat("interrupted")
			ip.wg.Done()
			return
		case img, ok := <-ip.down.Next():
			if !ok {
				// Downloader channel with images is closed due to reaching Inputer EOF. Stop the runner.
				logstat("reached EOF")
				ip.wg.Done()
				return
			}

			url := img.URL()
			size := len(img.Bytes())

			t := time.Now()
			res, err := ip.counter.Count(img)
			img.Reset() // return []byte to the pool.
			if err != nil {
				log.Error().Str("url", url).Str("errmsg", err.Error()).Msg("counting failed")
				break
			}
			log.Debug().Str("url", url).Str("dur", time.Since(t).String()).Str("res", res.Result()).Msg("image processed")

			per100b += size
			if size > msize {
				msize = size
			}

			cnt++
			if cnt%100 == 0 {
				totalb += per100b
				logstat("+100 processed")
				started100 = time.Now()
				per100b = 0
			}
			if err := ip.output.Save(res); err != nil {
				log.Error().Str("url", url).Str("errmsg", err.Error()).Msg("result saving failed")
				break
			}
		}
	}
}

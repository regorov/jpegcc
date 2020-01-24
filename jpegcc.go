// Package jpegcc provides functionality for batch jpeg file processing.
package jpegcc

import (
	"context"
)

// Resulter is the interface that wraps Result and Header methods.
//
// Result returns string representation of processing result.
//
// Header returns header if output format expects header (e.g. CSV file format).
// If output format does not requires header, method implementation can return
// empty string.
type Resulter interface {
	Result() string
	Header() string
}

// Outputer is the interface that wraps Save and Close method,
//
// Save receives Resulter to be written to the output.
//
// Close flushes output buffer and closes output.
type Outputer interface {
	Save(Resulter) error
	Close() error
}

// Counter is the interface that wraps the basic Count method,
//
// Count receives Imager, process it in accordance to implementation and returns Resulter or
// error if processing failed.
type Counter interface {
	Count(Imager) (Resulter, error)
}

// Inputer is the interface that wraps the basic Next method.
//
// Next returns channel of URL's read from input. Channel closes
// when input EOF is reached.
type Inputer interface {
	Next() <-chan string
}

// Downloader is the interface that groups methods Download and Next.
//
// Download downloads image addressed by url and returns it wrapped into Imager.
//
// Next returns channel of Imager downloaded and ready to be processed. Channel
// closes when nothing to download.
type Downloader interface {
	Download(ctx context.Context, url string) (Imager, error)
	Next() <-chan Imager
}

// Imager is the interface that groups methods to deal with
// downloaded image.
//
// Bytes returns downloaded image as []byte.
//
// Reset releases []byte of HTTP Body. Do not call Bytes() after
// calling Reset.
//
// URL returns the URL of downloaded image.
type Imager interface {
	Bytes() []byte
	Reset()
	URL() string
}

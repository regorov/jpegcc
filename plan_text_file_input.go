package jpegcc

import (
	"bufio"
	"context"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// PlainTextFileInput implements interface Inputer and provides jpeg urls from plain text file.
type PlainTextFileInput struct {
	line        chan string
	eof         chan struct{}
	log         zerolog.Logger
	linesPassed int
}

// NewPlainTextFileInput returns new instance of PlainTextFileInput.
func NewPlainTextFileInput(l zerolog.Logger) *PlainTextFileInput {
	return &PlainTextFileInput{log: l.With().Str("component", "inputer").Logger(), line: make(chan string), eof: make(chan struct{})}
}

// Start opens an input file in read only mode and starts runner (separate goroutine) of line by
// line reading to chan string. Returns error if could not open a file.
func (inp *PlainTextFileInput) Start(ctx context.Context, fname string) error {

	file, err := os.OpenFile(fname, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}

	go func() {
		go inp.runner(ctx, file)

		<-inp.eof // waiting reaching end of file.
		close(inp.line)
		_ = file.Close() // we can ignore file.Close() error because of readonly mode.
	}()
	return nil
}

func (inp *PlainTextFileInput) runner(ctx context.Context, file *os.File) {

	scanner := bufio.NewScanner(file)
	for {
		select {
		case <-ctx.Done():
			inp.eof <- struct{}{}
			return
		default:
			if scanner.Scan() {
				s := strings.TrimSpace(scanner.Text())
				if len(s) < 8 {
					// ignore url's with length less then "http://1".
					break
				}

				// catching ctx.Done() while line chan is full.
				select {
				case inp.line <- s:
					inp.linesPassed++
					break
				case <-ctx.Done():
					inp.eof <- struct{}{}
					return
				}
				break
			}

			if scanner.Err() != nil {
				inp.log.Error().Str("errmsg", scanner.Err().Error()).Msg("scanner failed")
			}
			inp.eof <- struct{}{}
			return
		}
	}
}

// Next returns chan with urls read from file.
func (inp *PlainTextFileInput) Next() <-chan string {
	return inp.line
}

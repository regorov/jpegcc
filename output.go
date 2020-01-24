package jpegcc

import (
	"os"
	"strings"
	"sync"
)

// BufferedCSV implements Outputer interface. CSV file with write buffer.
type BufferedCSV struct {
	mux sync.Mutex
	// TODO: buf as slice of string is not correct approach.
	buf                 []string
	file                *os.File
	isHeadWriteRequired bool
}

// DefaultBufferLen defines default output buffer length.
const DefaultBufferLen = 10

// NewBufferedCSV returns new BufferedCSV instance. If size < 2, DefaultBufferLen (10) will be assigned.
func NewBufferedCSV(size int) *BufferedCSV {
	if size < 2 {
		size = DefaultBufferLen
	}
	return &BufferedCSV{buf: make([]string, 0, size)}
}

// Open creates file or appends if file is exist. CSV header writes only into empty file.
func (out *BufferedCSV) Open(fname string) error {

	var err error
	out.file, err = os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	flen, err := out.file.Seek(0, 2) // to the end of the file
	if err != nil {
		_ = out.file.Close()
		return err
	}

	out.isHeadWriteRequired = (flen == 0)

	return nil
}

// Save adds Resulter to the buffer and flushes buffer to the file if buffer length reached the limit.
func (out *BufferedCSV) Save(res Resulter) error {

	out.mux.Lock()
	defer out.mux.Unlock() // defer slows execution. Can be refactored.

	if out.file == nil {
		// ignore, if Processer called Inputer.Save() later than Inputer.Close() was called.
		return nil
	}

	if out.isHeadWriteRequired {
		// applies only at the first Save() call.
		out.buf = append(out.buf, res.Header())
		out.isHeadWriteRequired = false
	}

	out.buf = append(out.buf, res.Result())
	if len(out.buf) < cap(out.buf) {
		return nil
	}

	if _, err := out.file.WriteString(out.csvLines(out.buf)); err != nil {
		return err
	}

	out.buf = out.buf[0:0:cap(out.buf)]
	return nil
}

func (out *BufferedCSV) csvLines(r []string) string {
	if len(r) == 0 {
		return ""
	}

	return strings.Join(out.buf, "")
}

// Close flushes to the output file unsaved buffer and closes file.
func (out *BufferedCSV) Close() error {
	out.mux.Lock()
	defer out.mux.Unlock()

	s := out.csvLines(out.buf)

	_, err := out.file.WriteString(s)
	if err == nil {
		err = out.file.Close()
	} else {
		// return error related to WriteString
		_ = out.file.Close()
	}

	out.file = nil
	return err
}

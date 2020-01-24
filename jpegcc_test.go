package jpegcc_test

import (
	"context"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/regorov/jpegcc"
	"github.com/rs/zerolog"
)

const (
	assetPath    = "./assets"
	fileListener = "127.0.0.1:8091"
	baseURL      = "http://" + fileListener
)

var files = map[string]struct {
	status       int // http status code
	isExpected   bool
	isDownloaded bool
}{
	"404.jpeg":    {status: 404},
	"empty1.jpeg": {status: 204},
	"empty.jpeg":  {status: 200},
	"1x1.jpeg":    {status: 200, isExpected: true},
	"1x2.jpeg":    {status: 200, isExpected: true},
	"1x3.jpeg":    {status: 200, isExpected: true},
	"10x10.jpeg":  {status: 200, isExpected: true},
}

func init() {

	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			p := html.EscapeString(r.URL.Path)
			f, ok := files[p[1:]]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if f.status != 200 {
				w.WriteHeader(f.status)
				return
			}
			http.ServeFile(w, r, assetPath+p)
		})
		if err := http.ListenAndServe(fileListener, nil); err != nil {
			log.Fatalf("http listener start failed: %s", err.Error())
		}
	}()
}

func TestRBG_String(t *testing.T) {
	var tbl = []struct {
		color  jpegcc.RGB
		result string
	}{
		{0xFFFFFF, "#ffffff"}, {0, "#000000"}, {0xAA, "#0000aa"},
		{0x11223344, "#223344"}, {0x00AA00, "#00aa00"},
	}

	for i := range tbl {
		if res := tbl[i].color.String(); res != tbl[i].result {
			t.Errorf("case %d failed. Input: %x; got: %s, expected: %s", i, uint32(tbl[i].color), res, tbl[i].result)
		}
	}
}

func TestToRGB(t *testing.T) {
	var tbl = []struct {
		r, g, b uint32
		result  jpegcc.RGB
	}{{0xFF, 0xAA, 0xBB, 0xFFAABB}}

	for i := range tbl {
		if res := jpegcc.ToRGB(tbl[i].r, tbl[i].g, tbl[i].b); res != tbl[i].result {
			t.Errorf("case %d failed. Input (r,g,b): %x,%x,%x, got %x, expected: %x",
				i,
				tbl[i].r,
				tbl[i].g,
				tbl[i].b,
				res,
				tbl[i].result)
		}
	}
}

func TestResult_Result(t *testing.T) {
	var tbl = []struct {
		val    jpegcc.Result
		result string
	}{
		{val: jpegcc.Result{
			URL:    "http://reddit.com/img/aaa.jpg",
			Colors: [3]jpegcc.RGB{0xFFAABB, 0xCCBBEE, 0x112233}},
			result: string(`"http://reddit.com/img/aaa.jpg","#ffaabb","#ccbbee","#112233"`) + "\n"},
	}

	for i := range tbl {
		if res := tbl[i].val.Result(); res != tbl[i].result {
			t.Errorf("case %d failed. Got: %s, expected: %s", i, res, tbl[i].result)
		}
	}
}

func TestPlainTextFileInput_Next(t *testing.T) {

	fname := filepath.Join(assetPath, "input.txt")

	buf, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatalf("input file reading failed: %s", err.Error())
	}

	urls := strings.Split(string(buf), "\n")

	input := jpegcc.NewPlainTextFileInput(zerolog.Logger{})
	ctx, cancel := context.WithCancel(context.Background())
	if err := input.Start(ctx, fname); err != nil {
		t.Fatalf("inputer start failed. Details: %s", err.Error())
	}

	i := 0
	fi := 0
	for u := range input.Next() {
		fi++
		// input.Next() does not returns lines less then 8 chars.
		if len(urls[i]) < 8 {
			i++
		}

		if u != urls[i] {
			t.Fatalf("inconsistend reading. Got %s, expeted %s, line: %d", u, urls[i], fi)
			break
		}
		i++
		// test input reading cancellation.
		if len(urls)/2 == i {
			break
		}
	}

	cancel()
	for range input.Next() {
		i++
	}

	if i > len(urls)/2 {
		t.Errorf("input cancelation failed")
	}

}

type inputMock struct {
	line chan string
}

func newInputMock() jpegcc.Inputer {
	mock := inputMock{line: make(chan string)}

	go func() {
		for fname := range files {
			mock.line <- baseURL + "/" + fname
		}
		// adds to the input channel invalid URL. This URL should not be processed by
		// downloader.
		mock.line <- "http://127.0.0/wrong.url.jpeg"
		close(mock.line)
	}()

	return &mock
}

func (mock *inputMock) Next() <-chan string {
	return mock.line
}

func TestMediaDownloader(t *testing.T) {

	time.Sleep(time.Second)

	// TODO: ensure that ListenAndServe starts before down.Next() call.
	down := jpegcc.NewMediaDownloader(zerolog.New(os.Stdout), newInputMock())

	down.SetMaxConnsPerHost(1)
	down.SetReadTimeout(10 * time.Second)
	go func() {
		down.Start(context.Background(), 2)
	}()

	out := jpegcc.NewBufferedCSV(3)
	err := out.Open("./result.test")
	if err != nil {
		t.Fatalf("open file failed: %s", err.Error())
		return
	}

	for u := range down.Next() {
		pu, err := url.Parse(u.URL())
		if err != nil {
			t.Errorf("returns invalid URL. Got: %s", u.URL())
		}
		fname := pu.EscapedPath()[1:]

		s, ok := files[fname]
		if ok {
			s.isDownloaded = true
			files[fname] = s
		}
		u.Reset()
	}

	for f, s := range files {
		if s.isExpected != s.isDownloaded {
			t.Errorf("downloader failed. File: %s, %v", f, s)
		}
	}

}
func TestProcessor_CounterPix(t *testing.T) {

	// TODO: ensure that ListenAndServe starts before down.Next() call.
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	down := jpegcc.NewMediaDownloader(zerolog.New(os.Stdout), newInputMock())

	down.SetMaxConnsPerHost(1)
	down.SetReadTimeout(10 * time.Second)
	go func() {
		down.Start(context.Background(), 2)
	}()

	out := jpegcc.NewBufferedCSV(3)
	err := out.Open("./result.test")
	if err != nil {
		t.Fatalf("open file failed: %s", err.Error())
	}

	counter := jpegcc.NewCounterPix()
	imgproc := jpegcc.NewImageProcessor(zerolog.New(os.Stdout), down, out, counter)
	imgproc.Start(context.Background(), 1)

	if err := out.Close(); err != nil {
		t.Fatalf("output file close failed: %s", err.Error())
	}

	if err := os.Remove("./result.test"); err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("delete ./result.test failed: %s", err.Error())
		}
	}
}

func count(t *testing.T, counter jpegcc.Counter) {

	// TODO: ensure that ListenAndServe starts before down.Next() call.
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	down := jpegcc.NewMediaDownloader(zerolog.New(os.Stdout), newInputMock())

	down.SetMaxConnsPerHost(1)
	down.SetReadTimeout(10 * time.Second)
	go func() {
		down.Start(context.Background(), 2)
	}()

	out := jpegcc.NewBufferedCSV(3)
	err := out.Open("./result.test")
	if err != nil {
		t.Fatalf("open file failed: %s", err.Error())
	}

	imgproc := jpegcc.NewImageProcessor(zerolog.New(os.Stdout), down, out, counter)
	imgproc.Start(context.Background(), 1)

	if err := out.Close(); err != nil {
		t.Fatalf("output file close failed: %s", err.Error())
	}

	if err := os.Remove("./result.test"); err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("delete ./result.test failed: %s", err.Error())
		}
	}
}
func TestCounterDfault(t *testing.T) {
	count(t, jpegcc.NewCounterDefault())
}

func TestCounterPix(t *testing.T) {
	count(t, jpegcc.NewCounterPix())
}

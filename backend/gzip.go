package backend

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(nil)
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gw *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.gw.Write(b)
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gw := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(gw)
		gw.Reset(w)

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")

		gzw := &gzipResponseWriter{ResponseWriter: w, gw: gw}
		next.ServeHTTP(gzw, r)

		gw.Flush()
		gw.Close()
	})
}

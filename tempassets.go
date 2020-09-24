package main

import (
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/lucsky/cuid"
	cmap "github.com/orcaman/concurrent-map"
)

var tempAssets = cmap.New()

func serveTempAssets() {
	router.PathPrefix("/tempasset/").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			name := r.URL.Path[11:]
			mimeType := mime.TypeByExtension(filepath.Base(name))
			w.Header().Set("Content-Type", mimeType)

			if val, ok := tempAssets.Get(name); ok {
				b, _ := val.([]byte)
				w.Write(b)
			} else {
				http.Error(w, "file not found", 404)
			}
		},
	)
}

func tempAssetURL(ext string, data []byte) *url.URL {
	name := cuid.Slug() + ext
	tempAssets.Set(name, data)
	u, _ := url.Parse(s.ServiceURL + "/tempasset/" + name)

	go func(name string) {
		time.Sleep(5 * time.Minute)
		tempAssets.Remove(name)
	}(name)

	return u
}

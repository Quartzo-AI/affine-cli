package cli

import (
	"affine-pp-cli/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadCanvasImageUsesMultipartGraphQL(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" || r.URL.Path != "/graphql" {
			t.Error("wrong auth or endpoint")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
		}
		if r.FormValue("map") != `{"0":["variables.blob"]}` {
			t.Error("wrong upload map")
		}
		f, h, err := r.FormFile("0")
		if err != nil {
			t.Error(err)
		} else {
			defer f.Close()
			if h.Header.Get("Content-Type") != "image/png" {
				t.Error("wrong mime")
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"setBlob":"blob-id"}}`))
	}))
	defer s.Close()
	id, err := uploadCanvasImage(&config.Config{BaseURL: s.URL, AffineToken: "test"}, "workspace", "image.png", "image/png", []byte("image"))
	if err != nil || id != "blob-id" {
		t.Fatalf("%q %v", id, err)
	}
}

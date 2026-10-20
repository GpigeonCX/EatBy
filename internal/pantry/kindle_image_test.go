package pantry

import (
	"image/png"
	"os"
	"testing"
)

func TestDeviceImageAuthorization(t *testing.T) {
	server, _ := newTestServer(t)
	anon := &testClient{t: t, server: server}
	assertStatus(t, anon.request("GET", "/api/v1/display/invalid/image.png", ""), 401)
}
func TestDeviceImageRender(t *testing.T) {
	if os.Getenv("EATBY_KINDLE_FONT") == "" {
		t.Skip("requires Chinese font")
	}
	server, _ := newTestServer(t)
	owner := registerHousehold(t, server, "image_owner", "家庭")
	token := decodeBody[map[string]string](t, owner.request("POST", "/api/v1/display-tokens", `{"name":"Kindle"}`))
	anon := &testClient{t: t, server: server}
	path := "/api/v1/display/" + token["token"] + "/image.png"
	for _, direction := range []string{"portrait", "landscape_cw", "landscape_ccw"} {
		assertStatus(t, owner.request("PUT", "/api/v1/display-tokens/"+token["id"]+"/schedule", `{"timezone":"Asia/Shanghai","enabled":false,"windows":[],"orientation":"`+direction+`"}`), 200)
		w := anon.request("GET", path, "")
		assertStatus(t, w, 200)
		if w.Header().Get("Content-Type") != "image/png" || w.Header().Get("X-Eatby-Next-Refresh") != "0" {
			t.Fatal(w.Header())
		}
		cfg, err := png.DecodeConfig(w.Body)
		if err != nil || cfg.Width != 600 || cfg.Height != 800 {
			t.Fatal(cfg, err)
		}
	}
	assertStatus(t, owner.request("DELETE", "/api/v1/display-tokens/"+token["id"], ""), 200)
	assertStatus(t, anon.request("GET", path, ""), 401)
}

package pantry

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

type testClient struct {
	t      *testing.T
	server http.Handler
	cookie *http.Cookie
}

func newTestClient(t *testing.T) *testClient {
	t.Helper()
	store, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	web := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	var filesystem fs.FS = web
	return &testClient{t: t, server: NewServer(store, filesystem)}
}

func (c *testClient) request(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if c.cookie != nil {
		r.AddCookie(c.cookie)
	}
	w := httptest.NewRecorder()
	c.server.ServeHTTP(w, r)
	if cookies := w.Result().Cookies(); len(cookies) > 0 && cookies[0].Name == "pantry_session" && cookies[0].MaxAge >= 0 {
		c.cookie = cookies[0]
	}
	return w
}

func decodeBody[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return value
}

func setupTestHousehold(t *testing.T, c *testClient) {
	t.Helper()
	w := c.request(http.MethodPost, "/api/v1/setup", `{"name":"测试家庭","password":"testpass123","display_name":"户主"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup: %d %s", w.Code, w.Body.String())
	}
}

func TestInventoryOpeningExpiryAndDisplay(t *testing.T) {
	c := newTestClient(t)
	setupTestHousehold(t, c)

	locations := decodeBody[[]Location](t, c.request(http.MethodGet, "/api/v1/locations/", ""))
	productResponse := c.request(http.MethodPost, "/api/v1/products/", `{"name":"鲜牛奶","category":"乳制品","default_unit":"盒","tracking_mode":"quantity","low_threshold":1,"after_open_days":3,"barcode":"6901","favorite":true}`)
	if productResponse.Code != http.StatusCreated {
		t.Fatal(productResponse.Body.String())
	}
	product := decodeBody[Product](t, productResponse)

	batchJSON := `{"product_id":"` + product.ID + `","location_id":"` + locations[0].ID + `","quantity":2,"expiry_date":"2099-12-31","note":""}`
	batchResponse := c.request(http.MethodPost, "/api/v1/batches/", batchJSON)
	if batchResponse.Code != http.StatusCreated {
		t.Fatal(batchResponse.Body.String())
	}
	batch := decodeBody[Batch](t, batchResponse)
	opened := decodeBody[Batch](t, c.request(http.MethodPost, "/api/v1/batches/"+batch.ID+"/open", `{}`))
	if opened.EffectiveExpiry == nil || opened.ExpiryStatus != "soon" {
		t.Fatalf("expected opening expiry to be soon, got %#v", opened)
	}

	tokenResponse := c.request(http.MethodPost, "/api/v1/display-tokens", `{"name":"Kindle"}`)
	token := decodeBody[map[string]string](t, tokenResponse)["token"]
	c.cookie = nil
	display := c.request(http.MethodGet, "/api/v1/display/"+token, "")
	if display.Code != http.StatusOK || !strings.Contains(display.Body.String(), "鲜牛奶") {
		t.Fatalf("display: %d %s", display.Code, display.Body.String())
	}
}

func TestInviteIsOneTime(t *testing.T) {
	c := newTestClient(t)
	setupTestHousehold(t, c)
	invite := decodeBody[map[string]any](t, c.request(http.MethodPost, "/api/v1/invites", `{}`))
	token := invite["token"].(string)

	member := newTestClientAgainst(c)
	first := member.request(http.MethodPost, "/api/v1/invites/accept", `{"token":"`+token+`","display_name":"家人"}`)
	if first.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", first.Code, first.Body.String())
	}
	second := c.request(http.MethodPost, "/api/v1/invites/accept", `{"token":"`+token+`","display_name":"另一人"}`)
	if second.Code != http.StatusBadRequest {
		t.Fatalf("invite reuse should fail: %d", second.Code)
	}
}

func TestBatchUsesOptimisticVersion(t *testing.T) {
	c := newTestClient(t)
	setupTestHousehold(t, c)
	locations := decodeBody[[]Location](t, c.request(http.MethodGet, "/api/v1/locations/", ""))
	product := decodeBody[Product](t, c.request(http.MethodPost, "/api/v1/products/", `{"name":"酱油","category":"调味品","default_unit":"瓶","tracking_mode":"quantity","low_threshold":1,"after_open_days":90,"favorite":false}`))
	created := decodeBody[Batch](t, c.request(http.MethodPost, "/api/v1/batches/", `{"product_id":"`+product.ID+`","location_id":"`+locations[0].ID+`","quantity":2,"note":""}`))
	body := `{"product_id":"` + product.ID + `","location_id":"` + locations[0].ID + `","quantity":1,"note":"第一次修改","version":` + strings.TrimSpace(jsonNumber(created.Version)) + `}`
	first := c.request(http.MethodPut, "/api/v1/batches/"+created.ID, body)
	if first.Code != http.StatusOK {
		t.Fatalf("first update: %d %s", first.Code, first.Body.String())
	}
	second := c.request(http.MethodPut, "/api/v1/batches/"+created.ID, body)
	if second.Code != http.StatusConflict {
		t.Fatalf("stale update should conflict: %d %s", second.Code, second.Body.String())
	}
}

func jsonNumber(value int) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func newTestClientAgainst(parent *testClient) *testClient {
	return &testClient{t: parent.t, server: parent.server}
}

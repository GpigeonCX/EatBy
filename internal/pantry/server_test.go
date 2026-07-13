package pantry

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

type testClient struct {
	t      *testing.T
	server http.Handler
	cookie *http.Cookie
}

func newTestServer(t *testing.T) (http.Handler, *Store) {
	t.Helper()
	store, e := Open(t.TempDir() + "/test.db")
	if e != nil {
		t.Fatal(e)
	}
	if e = store.BootstrapAdmin("admin", "admin-secret-123"); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { store.Close() })
	var web fs.FS = fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	return NewServer(store, web), store
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
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "eatby_session" && cookie.MaxAge >= 0 {
			c.cookie = cookie
		}
	}
	return w
}
func decodeBody[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), e)
	}
	return v
}
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status=%d want=%d body=%s", w.Code, want, w.Body.String())
	}
}
func registerHousehold(t *testing.T, server http.Handler, username, household string) *testClient {
	t.Helper()
	admin := &testClient{t: t, server: server}
	assertStatus(t, admin.request("POST", "/api/v1/auth/login", `{"username":"admin","password":"admin-secret-123"}`), 200)
	code := decodeBody[map[string]any](t, admin.request("POST", "/api/v1/admin/signup-codes", `{}`))["token"].(string)
	client := &testClient{t: t, server: server}
	body := `{"code":"` + code + `","username":"` + username + `","password":"household-pass-123","display_name":"户主","household_name":"` + household + `"}`
	assertStatus(t, client.request("POST", "/api/v1/register", body), 201)
	return client
}

func TestHouseholdIsolationAndOpeningExpiry(t *testing.T) {
	server, _ := newTestServer(t)
	a := registerHousehold(t, server, "family_a", "甲家庭")
	b := registerHousehold(t, server, "family_b", "乙家庭")
	locA := decodeBody[[]Location](t, a.request("GET", "/api/v1/locations/", ""))[0]
	locB := decodeBody[[]Location](t, b.request("GET", "/api/v1/locations/", ""))[0]
	pbody := `{"name":"鲜牛奶","category":"乳制品","default_unit":"盒","tracking_mode":"quantity","low_threshold":1,"after_open_days":3,"barcode":"6901","favorite":true}`
	pa := decodeBody[Product](t, a.request("POST", "/api/v1/products/", pbody))
	pb := decodeBody[Product](t, b.request("POST", "/api/v1/products/", pbody))
	if pa.ID == pb.ID {
		t.Fatal("ids should differ")
	}
	batch := decodeBody[Batch](t, a.request("POST", "/api/v1/batches/", `{"product_id":"`+pa.ID+`","location_id":"`+locA.ID+`","quantity":2,"expiry_date":"2099-12-31","note":""}`))
	opened := decodeBody[Batch](t, a.request("POST", "/api/v1/batches/"+batch.ID+"/open", `{}`))
	if opened.EffectiveExpiry == nil || opened.ExpiryStatus != "soon" {
		t.Fatalf("unexpected expiry %#v", opened)
	}
	assertStatus(t, b.request("DELETE", "/api/v1/products/"+pa.ID, ""), 404)
	assertStatus(t, b.request("POST", "/api/v1/shopping/from-product/"+pa.ID, `{}`), 404)
	_ = locB
	display := decodeBody[map[string]string](t, a.request("POST", "/api/v1/display-tokens", `{"name":"Kindle"}`))
	anonymous := &testClient{t: t, server: server}
	w := anonymous.request("GET", "/api/v1/display/"+display["token"], "")
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "鲜牛奶") {
		t.Fatal("display missing household data")
	}
}

func TestPlatformAndHouseholdInvitesAreOneTime(t *testing.T) {
	server, _ := newTestServer(t)
	owner := registerHousehold(t, server, "owner_a", "甲家庭")
	invite := decodeBody[map[string]any](t, owner.request("POST", "/api/v1/household-invites", `{}`))
	token := invite["token"].(string)
	member := &testClient{t: t, server: server}
	body := `{"username":"member_a","password":"member-pass-123","display_name":"家人"}`
	assertStatus(t, member.request("POST", "/api/v1/household-invites/"+token+"/accept", body), 200)
	assertStatus(t, (&testClient{t: t, server: server}).request("POST", "/api/v1/household-invites/"+token+"/accept", `{"username":"member_b","password":"member-pass-123","display_name":"另一人"}`), 400)
	members := decodeBody[[]UserSummary](t, owner.request("GET", "/api/v1/members", ""))
	if len(members) != 2 {
		t.Fatalf("members=%d", len(members))
	}
}

func TestVersionConflictAndSuspendedHousehold(t *testing.T) {
	server, _ := newTestServer(t)
	client := registerHousehold(t, server, "owner_c", "丙家庭")
	me := decodeBody[map[string]any](t, client.request("GET", "/api/v1/me", ""))
	hid := me["household_id"].(string)
	loc := decodeBody[[]Location](t, client.request("GET", "/api/v1/locations/", ""))[0]
	p := decodeBody[Product](t, client.request("POST", "/api/v1/products/", `{"name":"酱油","category":"调味品","default_unit":"瓶","tracking_mode":"quantity","low_threshold":1,"after_open_days":90,"favorite":false}`))
	created := decodeBody[Batch](t, client.request("POST", "/api/v1/batches/", `{"product_id":"`+p.ID+`","location_id":"`+loc.ID+`","quantity":2,"note":""}`))
	body := `{"product_id":"` + p.ID + `","location_id":"` + loc.ID + `","quantity":1,"note":"修改","version":` + strconv.Itoa(created.Version) + `}`
	assertStatus(t, client.request("PUT", "/api/v1/batches/"+created.ID, body), 200)
	assertStatus(t, client.request("PUT", "/api/v1/batches/"+created.ID, body), 409)
	admin := &testClient{t: t, server: server}
	assertStatus(t, admin.request("POST", "/api/v1/auth/login", `{"username":"admin","password":"admin-secret-123"}`), 200)
	assertStatus(t, admin.request("GET", "/api/v1/dashboard", ""), 403)
	assertStatus(t, admin.request("PUT", "/api/v1/admin/households/"+hid+"/status", `{"status":"suspended"}`), 200)
	assertStatus(t, client.request("GET", "/api/v1/dashboard", ""), 403)
}

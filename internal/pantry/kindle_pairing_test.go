package pantry

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKindlePairing(t *testing.T) {
	server, store := newTestServer(t)
	owner := registerHousehold(t, server, "pair_owner", "配对家庭")
	anon := &testClient{t: t, server: server}
	start := anon.request("POST", "/api/v1/kindle-pairings", "")
	assertStatus(t, start, 200)
	fields := strings.Fields(start.Body.String())
	if len(fields) != 2 || len(fields[0]) != 9 || len(fields[1]) != 43 {
		t.Fatal("invalid pairing response")
	}
	poll := func(secret string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/kindle-pairings/status", nil)
		r.Header.Set("Authorization", "Bearer "+secret)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		return w
	}
	assertStatus(t, poll(fields[1]), 202)
	assertStatus(t, poll("wrong"), 410)
	body := `{"code":"` + fields[0] + `","name":"客厅"}`
	assertStatus(t, anon.request("POST", "/api/v1/kindle-pairings/claim", body), 401)
	claimed := owner.request("POST", "/api/v1/kindle-pairings/claim", body)
	assertStatus(t, claimed, 201)
	id := decodeBody[map[string]string](t, claimed)["id"]
	assertStatus(t, poll(fields[1]), 200)
	assertStatus(t, owner.request("POST", "/api/v1/kindle-pairings/claim", body), 400)
	assertStatus(t, anon.request("GET", "/api/v1/display/"+fields[1]+"/config", ""), 200)
	assertStatus(t, owner.request("DELETE", "/api/v1/display-tokens/"+id, ""), 200)
	assertStatus(t, poll(fields[1]), 410)
	assertStatus(t, anon.request("GET", "/api/v1/display/"+fields[1]+"/config", ""), 401)
	start = anon.request("POST", "/api/v1/kindle-pairings", "")
	fields = strings.Fields(start.Body.String())
	if _, err := store.DB.Exec(`UPDATE kindle_pairings SET expires_at=0`); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, poll(fields[1]), 410)
	assertStatus(t, owner.request("POST", "/api/v1/kindle-pairings/claim", `{"code":"`+fields[0]+`"}`), 400)
}

package pantry

import (
	"encoding/json"
	"pantry/internal/kindle"
	"testing"
)

func TestKindleScheduleAPI(t *testing.T) {
	server, store := newTestServer(t)
	owner := registerHousehold(t, server, "kindle_owner", "家庭")
	other := registerHousehold(t, server, "other_owner", "其他家庭")
	anon := &testClient{t: t, server: server}
	d := decodeBody[map[string]string](t, owner.request("POST", "/api/v1/display-tokens", `{"name":"Kindle8"}`))
	if len(d["token"]) != 22 {
		t.Fatalf("expected 128-bit short token, got length %d", len(d["token"]))
	}
	assertStatus(t, anon.request("GET", "/d/"+d["token"], ""), 200)
	path := "/api/v1/display-tokens/" + d["id"] + "/schedule"
	device := "/api/v1/display/" + d["token"] + "/config"
	assertStatus(t, anon.request("GET", path, ""), 401)
	assertStatus(t, other.request("GET", path, ""), 404)
	defaultResponse := anon.request("GET", device, "")
	assertStatus(t, defaultResponse, 200)
	if defaultResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
	var initial struct {
		Schedule kindle.Schedule `json:"schedule"`
	}
	initial = decodeBody[struct {
		Schedule kindle.Schedule `json:"schedule"`
	}](t, defaultResponse)
	if !initial.Schedule.Enabled || len(initial.Schedule.Windows) != 2 {
		t.Fatal("unexpected defaults")
	}
	schedule := kindle.DefaultSchedule()
	schedule.Enabled = false
	schedule.Orientation = "landscape_cw"
	data, _ := json.Marshal(schedule)
	assertStatus(t, other.request("PUT", path, string(data)), 404)
	assertStatus(t, owner.request("PUT", path, string(data)), 200)
	response := decodeBody[map[string]any](t, anon.request("GET", device, ""))
	if response["layout_width"] != float64(800) || response["layout_height"] != float64(600) || response["width"] != float64(600) || response["height"] != float64(800) {
		t.Fatal("wrong layout/panel dimensions", response)
	}
	if response["schedule"].(map[string]any)["orientation"] != "landscape_cw" {
		t.Fatal("orientation not persisted")
	}
	if response["next_refresh_at"] != nil {
		t.Fatal("paused device has next refresh")
	}
	schedule.Enabled = true
	schedule.Windows[0].Every = 0
	data, _ = json.Marshal(schedule)
	assertStatus(t, owner.request("PUT", path, string(data)), 400)
	invite := decodeBody[map[string]any](t, owner.request("POST", "/api/v1/household-invites", "{}"))
	member := &testClient{t: t, server: server}
	assertStatus(t, member.request("POST", "/api/v1/household-invites/"+invite["token"].(string)+"/accept", `{"username":"kindle_member","password":"secret","display_name":"成员"}`), 200)
	assertStatus(t, member.request("GET", path, ""), 403)
	assertStatus(t, member.request("PUT", path, string(data)), 403)
	assertStatus(t, owner.request("DELETE", "/api/v1/display-tokens/"+d["id"], ""), 200)
	assertStatus(t, anon.request("GET", device, ""), 401)
	var count int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM display_schedules WHERE display_id=?`, d["id"]).Scan(&count); err != nil || count != 0 {
		t.Fatal("schedule not removed", count, err)
	}
}

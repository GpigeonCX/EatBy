package pantry

import (
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestInventoryQueriesWithBoundedPool(t *testing.T) {
	server, store := newTestServer(t)
	client := registerHousehold(t, server, "pool_test", "连接池测试")
	loc := decodeBody[[]Location](t, client.request("GET", "/api/v1/locations/", ""))[0]
	p := decodeBody[Product](t, client.request("POST", "/api/v1/products/", `{"name":"牛奶","default_unit":"盒","tracking_mode":"quantity","low_threshold":1}`))
	assertStatus(t, client.request("POST", "/api/v1/batches/", fmt.Sprintf(`{"product_id":%q,"location_id":%q,"quantity":2,"expiry_date":"2099-12-31"}`, p.ID, loc.ID)), 201)
	token := decodeBody[map[string]string](t, client.request("POST", "/api/v1/display-tokens", `{"name":"test"}`))["token"]
	paths := []string{"/api/v1/dashboard", "/api/v1/products/", "/api/v1/batches/", "/api/v1/display/" + token, "/healthz"}
	for _, size := range []int{1, 5} {
		t.Run(fmt.Sprintf("connections_%d", size), func(t *testing.T) {
			store.DB.SetMaxOpenConns(size)
			errors := make(chan string, 30)
			var wg sync.WaitGroup
			start := make(chan struct{})
			for i := 0; i < 30; i++ {
				wg.Add(1)
				go func(path string) {
					defer wg.Done()
					<-start
					r := httptest.NewRequest("GET", path, nil)
					r.AddCookie(client.cookie)
					w := httptest.NewRecorder()
					server.ServeHTTP(w, r)
					if w.Code != 200 {
						errors <- fmt.Sprintf("%s: %d %s", path, w.Code, w.Body.String())
					}
				}(paths[i%len(paths)])
			}
			close(start)
			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				// Unblock a regressed nested query so test cleanup itself cannot hang.
				store.DB.SetMaxOpenConns(100)
				<-done
				t.Fatal("queries stalled with a bounded pool")
			}
			close(errors)
			for err := range errors {
				t.Error(err)
			}
			if n := store.DB.Stats().InUse; n != 0 {
				t.Errorf("leaked connections: %d", n)
			}
		})
	}
}

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDocsSmoke(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	p := r.Group("/")
	p.Use(func(c *gin.Context) {
		c.Set("user", map[string]any{"id": int32(1), "role": "student"})
		c.Next()
	})
	registerGET(p, r)
	registerOther(p)
	registerDocs(r, r)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()

	if dump := os.Getenv("SMOKE_DUMP"); dump != "" {
		_ = os.WriteFile(dump, []byte(body), 0o644)
	}

	for _, path := range []string{"/info", "/developer/age", "/users/me", "/admin/assignments", "/api/teacher/schedule"} {
		if !strings.Contains(body, path) {
			t.Errorf("docs не содержит %s", path)
		}
	}

	var routes []docsEndpoint
	i := strings.Index(body, "var ROUTES = ")
	j := strings.Index(body[i:], ";\n")
	if err := json.Unmarshal([]byte(body[i+13:i+j]), &routes); err != nil {
		t.Fatalf("ROUTES JSON: %v", err)
	}

	var userMe *docsEndpoint
	for k := range routes {
		if routes[k].Path == "/users/me" {
			userMe = &routes[k]
		}
	}
	if userMe == nil {
		t.Fatal("/users/me нет в ROUTES")
	}

	wantMethods := map[string]bool{"PATCH": true, "DELETE": true}
	for _, m := range userMe.Methods {
		if m == "GET" {
			wantMethods["GET"] = false
		} else if !wantMethods[m] {
			t.Errorf("/users/me: неожиданный метод %s", m)
		}
	}
	if have := hasMethod(wantMethods); have {
		if !hasSliceMethod(userMe.Methods, "GET") {
			t.Errorf("/users/me должен поддерживать GET, методы: %v", userMe.Methods)
		}
	}

	info := findEndpoint(routes, "/info")
	if info == nil {
		t.Fatal("/info нет в ROUTES")
	}
	if len(info.Methods) != 1 || info.Methods[0] != "GET" {
		t.Errorf("/info методы = %v, ожидается только [GET]", info.Methods)
	}
	homework := findEndpoint(routes, "/homework")
	if homework == nil {
		t.Fatal("/homework нет в ROUTES")
	}
	if len(homework.Methods) != 1 || homework.Methods[0] != "GET" {
		t.Errorf("/homework методы = %v, ожидается только [GET]", homework.Methods)
	}
	selectClass := findEndpoint(routes, "/select_class")
	if selectClass == nil {
		t.Fatal("/select_class нет в ROUTES")
	}
	if len(selectClass.Methods) != 1 || selectClass.Methods[0] != "POST" {
		t.Errorf("/select_class методы = %v, ожидается только [POST]", selectClass.Methods)
	}
}

func hasMethod(m map[string]bool) bool {
	for _, v := range m {
		if v {
			return true
		}
	}
	return false
}

func hasSliceMethod(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func findEndpoint(routes []docsEndpoint, path string) *docsEndpoint {
	for k := range routes {
		if routes[k].Path == path {
			return &routes[k]
		}
	}
	return nil
}

package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"nytrpg/internal/store"
)

func newService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, []byte("secret"))
}

func call(h http.HandlerFunc, method string, body any) (*httptest.ResponseRecorder, map[string]any) {
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(method, "/", bytes.NewReader(b)))
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestSignupValidation(t *testing.T) {
	a := newService(t)
	cases := []struct {
		name, user, pass string
		code             int
	}{
		{"empty username", "", "pw", 400},
		{"blank username", "   ", "pw", 400},
		{"too long", strings.Repeat("a", 21), "pw", 400},
		{"no password", "bob", "", 400},
		{"ok", "bob", "pw", 200},
		{"20 characters is fine", strings.Repeat("é", 20), "pw", 200},
	}
	for _, c := range cases {
		rec, _ := call(a.HandleSignup, "POST", map[string]string{"username": c.user, "password": c.pass})
		if rec.Code != c.code {
			t.Errorf("%s: want %d, got %d %s", c.name, c.code, rec.Code, rec.Body)
		}
	}

	_, out := call(a.HandleSignup, "POST", map[string]string{"username": " bob ", "password": "pw"})
	if out["usernameAvailable"] != false {
		t.Fatalf("username is trimmed, so bob is taken: %v", out)
	}
	if rec, _ := call(a.HandleSignup, "GET", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET signup: %d", rec.Code)
	}
}

func TestLoginAndToken(t *testing.T) {
	a := newService(t)
	call(a.HandleSignup, "POST", map[string]string{"username": "alice", "password": "right"})

	_, out := call(a.HandleLogin, "POST", map[string]string{"username": "alice", "password": "wrong"})
	if out["validUser"] != false || out["jwt"] != "" {
		t.Fatalf("wrong password logged in: %v", out)
	}
	_, out = call(a.HandleLogin, "POST", map[string]string{"username": "nobody", "password": "right"})
	if out["validUser"] != false {
		t.Fatalf("unknown user logged in: %v", out)
	}

	_, out = call(a.HandleLogin, "POST", map[string]string{"username": "alice", "password": "right"})
	token, _ := out["jwt"].(string)
	if out["validUser"] != true || token == "" || out["username"] != "alice" {
		t.Fatalf("login failed: %v", out)
	}

	_, out = call(a.HandleToken, "POST", token)
	if out["validUser"] != true || out["username"] != "alice" {
		t.Fatalf("token not accepted: %v", out)
	}
	_, out = call(a.HandleToken, "POST", "not.a.token")
	if out["validUser"] != false {
		t.Fatalf("garbage token accepted: %v", out)
	}
}

func TestBadRequests(t *testing.T) {
	a := newService(t)
	for name, h := range map[string]http.HandlerFunc{"login": a.HandleLogin, "signup": a.HandleSignup, "token": a.HandleToken} {
		if rec, _ := call(h, "GET", nil); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s GET: %d", name, rec.Code)
		}
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest("POST", "/", strings.NewReader("{not json")))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s bad json: %d", name, rec.Code)
		}
	}
}

func TestVerifyTokenRejectsForgeries(t *testing.T) {
	a := newService(t)
	sign := func(method jwt.SigningMethod, key any, claims jwt.MapClaims) string {
		s, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	future := time.Now().Add(time.Hour).Unix()

	good := sign(jwt.SigningMethodHS256, []byte("secret"), jwt.MapClaims{"username": "alice", "exp": future})
	if u, ok := a.VerifyToken(good); !ok || u != "alice" {
		t.Fatalf("valid token rejected")
	}

	forged := map[string]string{
		"wrong key":       sign(jwt.SigningMethodHS256, []byte("other"), jwt.MapClaims{"username": "alice", "exp": future}),
		"expired":         sign(jwt.SigningMethodHS256, []byte("secret"), jwt.MapClaims{"username": "alice", "exp": time.Now().Add(-time.Hour).Unix()}),
		"other algorithm": sign(jwt.SigningMethodHS512, []byte("secret"), jwt.MapClaims{"username": "alice", "exp": future}),
		"alg none":        sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, jwt.MapClaims{"username": "alice", "exp": future}),
		// Used to panic on the type assertion
		"no username":      sign(jwt.SigningMethodHS256, []byte("secret"), jwt.MapClaims{"exp": future}),
		"numeric username": sign(jwt.SigningMethodHS256, []byte("secret"), jwt.MapClaims{"username": 5, "exp": future}),
		"empty":            "",
	}
	for name, token := range forged {
		if u, ok := a.VerifyToken(token); ok {
			t.Errorf("%s: accepted as %q", name, u)
		}
	}
}

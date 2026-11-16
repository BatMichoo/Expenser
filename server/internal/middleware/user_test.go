package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUsers struct {
	got  []string // "id name lang"
	fail bool
}

func (f *fakeUsers) EnsureUser(_ context.Context, id uuid.UUID, name, lang string) error {
	if f.fail {
		return errors.New("db down")
	}
	f.got = append(f.got, id.String()+" "+name+" "+lang)
	return nil
}

func serve(t *testing.T, store UserStore, dev bool, headers map[string]string) (*httptest.ResponseRecorder, User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var seen User
	r.Use(I18nMiddleware())
	r.GET("/", RequireUser(store, dev), func(c *gin.Context) {
		seen, _ = CurrentUser(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, seen
}

func TestRequireUserFromHeaders(t *testing.T) {
	id := uuid.New()
	users := &fakeUsers{}
	w, u := serve(t, users, false, map[string]string{
		"X-User-Id": id.String(), "X-User-Name": "%D0%9C%D0%B8%D1%80%D0%BE", "Cookie": "homeserver_lang=bg"})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, User{ID: id, Name: "Миро"}, u, "the name is percent-encoded UTF-8")
	assert.Equal(t, []string{id.String() + " Миро bg"}, users.got, "a row, with name and language for emails")

	w, u = serve(t, users, false, map[string]string{"X-User-Id": id.String(), "X-User-Name": "Bad%ZZ\r\nBcc: x"})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Bad%ZZBcc: x", u.Name, "taken as is when not encoded, and no line breaks")
}

func TestRequireUserWithoutHeaders(t *testing.T) {
	users := &fakeUsers{}
	for _, h := range []map[string]string{nil, {"X-User-Id": "not-a-uuid"}, {"X-User-Id": uuid.Nil.String()}} {
		w, _ := serve(t, users, false, h)
		assert.Equal(t, http.StatusUnauthorized, w.Code, h)
	}
	assert.Empty(t, users.got)

	// Dev mode has no nginx in front: the dev user.
	w, u := serve(t, users, true, nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, DevUserID, u.ID)
}

func TestRequireUserStoreDown(t *testing.T) {
	w, _ := serve(t, &fakeUsers{fail: true}, false, map[string]string{"X-User-Id": uuid.NewString()})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestLanguageFromSharedCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(I18nMiddleware())
	var lang string
	r.GET("/", func(c *gin.Context) { lang = c.GetString(LanguageCookieKey) })
	for cookie, want := range map[string]string{"": "en", "homeserver_lang=bg": "bg", "homeserver_lang=xx": "en", "lang=bg": "en"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		r.ServeHTTP(httptest.NewRecorder(), req)
		assert.Equal(t, want, lang, cookie)
	}
}

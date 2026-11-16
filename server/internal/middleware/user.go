package middleware

import (
	"context"
	"log"
	"net/http"
	"net/url"

	"expenser/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DevUserID is who a request without X-User-Id is in dev mode: the dev
// seed data's user (see db/seeds).
var DevUserID = uuid.MustParse("00000000-0000-0000-0000-00000000de00")

// UserStore keeps a row per person, made the first time they come.
type UserStore interface {
	EnsureUser(ctx context.Context, id uuid.UUID, name, lang string) error
}

// User is the logged-in person, as the home server sent them.
type User struct {
	ID   uuid.UUID
	Name string // display name, else username; may be ""
}

const userContextKey = "expenser.user"

// RequireUser takes the person from the headers the home server's nginx
// sets after its auth service checked the login: X-User-Id (a UUID) and
// X-User-Name (percent-encoded UTF-8). nginx replaces whatever the browser
// sent in them, and the app is reachable only through nginx, so they can
// be trusted. A request without them isn't logged in (401); nginx sends
// such people to the login page before they get here. In dev mode, with no
// nginx in front, they are DevUserID.
//
// Each person gets a row on their first request, and their name and
// language are kept on it (reminder emails, sent in the background, need
// both). Run I18nMiddleware first.
func RequireUser(store UserStore, devMode bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := userFromHeaders(c)
		if !ok && devMode {
			u, ok = User{ID: DevUserID, Name: "dev"}, true
		}
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if err := store.EnsureUser(c.Request.Context(), u.ID, u.Name, c.GetString(LanguageCookieKey)); err != nil {
			log.Printf("ensure user %s: %v", u.ID, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		SetUser(c, u)
		c.Next()
	}
}

func userFromHeaders(c *gin.Context) (User, bool) {
	id, err := uuid.Parse(c.GetHeader("X-User-Id"))
	if err != nil || id == uuid.Nil {
		return User{}, false
	}
	raw := c.GetHeader("X-User-Name")
	name, err := url.QueryUnescape(raw)
	if err != nil {
		name = raw
	}
	return User{ID: id, Name: models.CleanDisplayName(name)}, true
}

// SetUser records the logged-in person for the rest of the request.
func SetUser(c *gin.Context, u User) { c.Set(userContextKey, u) }

// CurrentUser is the person RequireUser found; ok is false on routes
// without it.
func CurrentUser(c *gin.Context) (User, bool) {
	v, ok := c.Get(userContextKey)
	u, _ := v.(User)
	return u, ok && u.ID != uuid.Nil
}

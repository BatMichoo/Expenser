package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/services/gemini"
	"expenser/internal/utilities"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSizesStore struct {
	owner   uuid.UUID
	rows    []models.GroceriesExpense
	setID   int
	setUnit string
	setSize float64
	skipped int
}

func (f *fakeSizesStore) GetUnsizedGroceries(context.Context, uuid.UUID) ([]models.GroceriesExpense, error) {
	return f.rows, nil
}

func (f *fakeSizesStore) CountUnsizedGroceries(context.Context, uuid.UUID) (int, error) {
	return len(f.rows), nil
}

func (f *fakeSizesStore) SetGrocerySize(_ context.Context, userID uuid.UUID, id int, unit string, size float64) error {
	if userID != f.owner {
		return fmt.Errorf("set: %w", sql.ErrNoRows)
	}
	f.setID, f.setUnit, f.setSize = id, unit, size
	return nil
}

func (f *fakeSizesStore) SkipGrocerySize(_ context.Context, userID uuid.UUID, id int) error {
	if userID != f.owner {
		return fmt.Errorf("skip: %w", sql.ErrNoRows)
	}
	f.skipped = id
	return nil
}

// sizesCall runs a sizes route as user, returning the recorder.
func sizesCall(t *testing.T, store *fakeSizesStore, user uuid.UUID, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	require.NoError(t, utilities.InitI18n())
	tmpl, err := utilities.ParseTemplates("../templates/*/*.html")
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.SetHTMLTemplate(tmpl)
	r.Use(func(c *gin.Context) {
		c.Set("lang", "en")
		services.SetClaims(c, &services.JWTClaims{UserID: user})
	})
	h := NewSizesHandler(store, nil)
	r.GET("/groceries/sizes", wrap(h.GetSizes))
	r.POST("/groceries/sizes/:id", wrap(h.SaveSize))
	r.POST("/groceries/sizes/:id/skip", wrap(h.SkipSize))

	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSaveSizeInvalid(t *testing.T) {
	user := uuid.New()
	for name, form := range map[string]url.Values{
		"bad unit":  {"unit": {"box"}, "unit_size": {"1"}},
		"zero size": {"unit": {"l"}, "unit_size": {"0"}},
		"no size":   {"unit": {"l"}},
		"not a num": {"unit": {"l"}, "unit_size": {"abc"}},
	} {
		t.Run(name, func(t *testing.T) {
			store := &fakeSizesStore{owner: user}
			w := sizesCall(t, store, user, "POST", "/groceries/sizes/5", form)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Zero(t, store.setID)
		})
	}
}

func TestSaveSizeNotOwned(t *testing.T) {
	store := &fakeSizesStore{owner: uuid.New()}
	w := sizesCall(t, store, uuid.New(), "POST", "/groceries/sizes/5", url.Values{"unit": {"l"}, "unit_size": {"1"}})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSaveSizeOK(t *testing.T) {
	user := uuid.New()
	store := &fakeSizesStore{owner: user, rows: make([]models.GroceriesExpense, 2)}
	w := sizesCall(t, store, user, "POST", "/groceries/sizes/5", url.Values{"unit": {"l"}, "unit_size": {"1"}})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 5, store.setID)
	assert.Equal(t, "l", store.setUnit)
	assert.Equal(t, 1.0, store.setSize)
	assert.Empty(t, w.Header().Get("HX-Trigger"), "nothing listens for an event")
	assert.Contains(t, w.Body.String(), `id="unsized-count"`)
	assert.Contains(t, w.Body.String(), `hx-swap-oob="true"`)
	assert.Contains(t, w.Body.String(), "2")
	assert.NotContains(t, w.Body.String(), `id="sizes-body"`, "rows left: the list stays")
}

func TestSkipSize(t *testing.T) {
	user := uuid.New()
	store := &fakeSizesStore{owner: user}
	w := sizesCall(t, store, user, "POST", "/groceries/sizes/7/skip", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 7, store.skipped)
	assert.Empty(t, w.Header().Get("HX-Trigger"))

	w = sizesCall(t, store, uuid.New(), "POST", "/groceries/sizes/7/skip", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// After the last row is saved or skipped, the list body is swapped out of
// band for the empty state, which drops the Suggest button.
func TestLastRowDoneShowsEmptyState(t *testing.T) {
	user := uuid.New()
	store := &fakeSizesStore{owner: user}
	w := sizesCall(t, store, user, "POST", "/groceries/sizes/7/skip", nil)
	require.Equal(t, http.StatusOK, w.Code)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(w.Body.String()))
	require.NoError(t, err)
	body := doc.Find(`#sizes-body[hx-swap-oob="true"]`)
	require.Equal(t, 1, body.Length(), w.Body.String())
	assert.Contains(t, body.Find(".sizes-empty").Text(), utilities.T("en", "sizes.all_sized"))
	assert.Equal(t, 0, doc.Find(".sizes-suggest").Length())
}

func TestGetSizes(t *testing.T) {
	user := uuid.New()
	store := &fakeSizesStore{owner: user, rows: []models.GroceriesExpense{{ID: 3, Product: "Мляко", Quantity: 1, CategoryName: "Dairy"}}}
	w := sizesCall(t, store, user, "GET", "/groceries/sizes", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Мляко")
	assert.Contains(t, w.Body.String(), `hx-post="/groceries/sizes/3"`)
}

type fakeSuggester struct {
	got []string
	out []gemini.SizeSuggestion
	err error
}

func (f *fakeSuggester) SuggestSizes(_ context.Context, names []string) ([]gemini.SizeSuggestion, error) {
	f.got = names
	return f.out, f.err
}

func suggestCall(t *testing.T, store *fakeSizesStore, ai Suggester) *httptest.ResponseRecorder {
	t.Helper()
	require.NoError(t, utilities.InitI18n())
	tmpl, err := utilities.ParseTemplates("../templates/*/*.html")
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.SetHTMLTemplate(tmpl)
	r.Use(func(c *gin.Context) {
		c.Set("lang", "en")
		services.SetClaims(c, &services.JWTClaims{UserID: store.owner})
	})
	h := NewSizesHandler(store, ai)
	r.POST("/groceries/sizes/suggest", wrap(h.Suggest))
	req := httptest.NewRequest(http.MethodPost, "/groceries/sizes/suggest", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSuggestPrefillsWithoutSaving(t *testing.T) {
	store := &fakeSizesStore{owner: uuid.New(), rows: []models.GroceriesExpense{{ID: 1, Product: "Мляко"}, {ID: 2, Product: "Нещо"}}}
	ai := &fakeSuggester{out: []gemini.SizeSuggestion{{Unit: "l", UnitSize: 1}, {}}}
	w := suggestCall(t, store, ai)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"Мляко", "Нещо"}, ai.got)
	assert.Zero(t, store.setID, "nothing saved")
	assert.Zero(t, store.skipped)
	assert.Equal(t, 1, strings.Count(w.Body.String(), `class="size-suggested"`), w.Body.String())
}

// Rows the parser already sized keep that size and are not sent to the AI.
func TestSuggestOnlyAsksRowsWithoutParserSize(t *testing.T) {
	store := &fakeSizesStore{owner: uuid.New(), rows: []models.GroceriesExpense{
		{ID: 1, Product: "Мляко 1л", Quantity: 1}, {ID: 2, Product: "Нещо", Quantity: 1}}}
	ai := &fakeSuggester{out: []gemini.SizeSuggestion{{Unit: "kg", UnitSize: 0.5}}}
	w := suggestCall(t, store, ai)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"Нещо"}, ai.got)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(w.Body.String()))
	require.NoError(t, err)
	parsed := doc.Find("#size-row-1")
	v, _ := parsed.Find(`input[name="unit_size"]`).Attr("value")
	assert.Equal(t, "1", v)
	u, _ := parsed.Find(`select[name="unit"] option[selected]`).Attr("value")
	assert.Equal(t, "l", u)
	assert.Equal(t, 0, parsed.Find(".size-suggested").Length())
	guessed := doc.Find("#size-row-2")
	v, _ = guessed.Find(`input[name="unit_size"]`).Attr("value")
	assert.Equal(t, "0.5", v)
	assert.Equal(t, 1, guessed.Find(".size-suggested").Length())
}

func TestSuggestAllParsedSkipsAI(t *testing.T) {
	ai := &fakeSuggester{}
	store := &fakeSizesStore{owner: uuid.New(), rows: []models.GroceriesExpense{{ID: 1, Product: "Мляко 1л", Quantity: 1}}}
	w := suggestCall(t, store, ai)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, ai.got)
}

func TestSuggestErrorShown(t *testing.T) {
	store := &fakeSizesStore{owner: uuid.New(), rows: []models.GroceriesExpense{{ID: 1, Product: "Мляко"}}}
	w := suggestCall(t, store, &fakeSuggester{err: gemini.ErrNotConfigured})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), html.EscapeString(utilities.T("en", "error.ai_not_configured")))
	assert.Contains(t, w.Body.String(), "Мляко", "list still shown")
}

func TestSuggestAsksAtMostLimitAndNotes(t *testing.T) {
	// Parser-sized rows do not count against the limit.
	rows := []models.GroceriesExpense{{ID: 100, Product: "Мляко 1л", Quantity: 1}, {ID: 101, Product: "Вода 0,5л", Quantity: 1}}
	for i := 1; i <= 45; i++ {
		rows = append(rows, models.GroceriesExpense{ID: i, Product: fmt.Sprintf("P%d", i)})
	}
	store := &fakeSizesStore{owner: uuid.New(), rows: rows}
	out := make([]gemini.SizeSuggestion, 40)
	ai := &fakeSuggester{out: out}
	w := suggestCall(t, store, ai)
	assert.Len(t, ai.got, 40)
	assert.Equal(t, "P40", ai.got[39])
	assert.Contains(t, w.Body.String(), "Suggested sizes for the first 40")
	assert.Contains(t, w.Body.String(), "P45", "the rest are still listed")
}

func TestSuggestEmptyListSkipsAI(t *testing.T) {
	ai := &fakeSuggester{}
	w := suggestCall(t, &fakeSizesStore{owner: uuid.New()}, ai)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, ai.got)
}

func TestSizesError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		key  string
	}{
		{"length mismatch", &gemini.ReplyError{Err: errors.New("mismatch")}, "error.sizes_not_suggested"},
		{"too long", &gemini.ReplyError{FinishReason: "MAX_TOKENS", Err: errors.New("cut")}, "error.sizes_too_many"},
		{"words", &gemini.ReplyError{Reply: "no idea", Err: errors.New("x")}, "error.sizes_ai_said"},
		{"refused", &gemini.ReplyError{FinishReason: "SAFETY", Err: errors.New("x")}, "error.sizes_ai_refused"},
		{"not configured", gemini.ErrNotConfigured, "error.ai_not_configured"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var httpErr *HTTPError
			require.ErrorAs(t, sizesError(tc.err), &httpErr)
			assert.Equal(t, tc.key, httpErr.MessageKey)
			assert.NotContains(t, httpErr.message("en"), "receipt")
		})
	}
}

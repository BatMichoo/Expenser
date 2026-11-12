package templates

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rule returns the declaration block of the first rule whose selector list is
// exactly sel.
func cssRule(t *testing.T, file, sel string) string {
	t.Helper()
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(sel) + ` \{([^}]*)\}`).FindSubmatch(b)
	require.NotNil(t, m, "rule %q not found", sel)
	return string(m[1])
}

func TestTrendSvgCapsHeight(t *testing.T) {
	r := cssRule(t, "../../static/css/styles.css", ".trend-svg")
	assert.Contains(t, r, "max-height: 360px")
	assert.Contains(t, cssRule(t, "../../static/css/styles.css", ".trend-svg text"), "font-size: 11px")
}

func TestSizeFieldsUseInputLook(t *testing.T) {
	for _, sel := range []string{".size-fields input,\n.size-fields select"} {
		r := cssRule(t, "../../static/css/styles.css", sel)
		assert.Contains(t, r, "background: var(--bg-light)")
		assert.Contains(t, r, "color: var(--text)")
	}
}

func TestSizeProductLeftAligned(t *testing.T) {
	assert.Contains(t, cssRule(t, "../../static/css/styles.css", ".size-product"), "text-align: left")
}

// On phones the 960x320 chart is drawn at least 600px wide (200px tall, text
// 18 user units = about 11px) and the card scrolls sideways.
func TestTrendSvgMobileMinWidth(t *testing.T) {
	b, err := os.ReadFile("../../static/css/mobile.css")
	require.NoError(t, err)
	css := string(b)
	m := regexp.MustCompile(`(?m)^\s*\.trend-svg \{([^}]*)\}`).FindStringSubmatch(css)
	require.NotNil(t, m)
	assert.Contains(t, m[1], "min-width: 600px")
	m = regexp.MustCompile(`(?m)^\s*\.trend-svg text \{([^}]*)\}`).FindStringSubmatch(css)
	require.NotNil(t, m)
	assert.Contains(t, m[1], "font-size: 18px")
	m = regexp.MustCompile(`(?m)^\s*\.trend-card \{([^}]*)\}`).FindStringSubmatch(css)
	require.NotNil(t, m)
	assert.Contains(t, m[1], "overflow-x: auto")
}

package gemini

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const receiptText = `Кауфланд България ЕООД енд Ко. КД
Картофи кг
 9,955 KG                           4,48 Б
Торбичка
 2 * 0,25                           0,50 Б`

func fakeExtract(text string, err error) func(context.Context, []byte) (string, error) {
	return func(context.Context, []byte) (string, error) { return text, err }
}

func TestReceiptPartsSendsPDFTextAsText(t *testing.T) {
	pdf := []byte("%PDF-1.4\n...")
	parts := receiptParts(context.Background(), pdf, fakeExtract(receiptText, nil))
	require.Len(t, parts, 2)
	text, _ := parts[1]["text"].(string)
	assert.Contains(t, text, "Картофи кг", "the receipt's own text, not a picture of a tall page")
	assert.Nil(t, parts[1]["inline_data"])
	assert.Contains(t, parts[0]["text"], "receipt")
}

func TestReceiptPartsFallsBackToThePDF(t *testing.T) {
	pdf := []byte("%PDF-1.4\n...")
	for name, extract := range map[string]func(context.Context, []byte) (string, error){
		"scanned (no text)":    fakeExtract("  \n\f ", nil),
		"pdftotext missing":    fakeExtract("", exec.ErrNotFound),
		"pdftotext fails":      fakeExtract("", errors.New("exit status 1")),
		"too little to be one": fakeExtract("Page 1", nil),
	} {
		t.Run(name, func(t *testing.T) {
			parts := receiptParts(context.Background(), pdf, extract)
			require.Len(t, parts, 2)
			inline, ok := parts[1]["inline_data"].(map[string]any)
			require.True(t, ok, "sent as the PDF itself")
			assert.Equal(t, "application/pdf", inline["mime_type"])
		})
	}
}

func TestReceiptPartsPhoto(t *testing.T) {
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}
	called := false
	parts := receiptParts(context.Background(), jpeg, func(context.Context, []byte) (string, error) {
		called = true
		return "", nil
	})
	assert.False(t, called, "only PDFs go through pdftotext")
	inline := parts[1]["inline_data"].(map[string]any)
	assert.Equal(t, "image/jpeg", inline["mime_type"])
}

// minimalPDF is a one-page PDF with a line of text, enough for pdftotext.
const minimalPDF = "%PDF-1.4\n" +
	"1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
	"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
	"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 300 100]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>endobj\n" +
	"4 0 obj<</Length 52>>stream\nBT /F1 12 Tf 10 50 Td (Milk 2 x 1.20 2.40) Tj ET\nendstream endobj\n" +
	"5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj\n" +
	"trailer<</Root 1 0 R>>\n%%EOF\n"

func TestPDFToTextWithPoppler(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext (poppler-utils) not installed")
	}
	text, err := pdfToText(context.Background(), []byte(minimalPDF))
	require.NoError(t, err)
	assert.Contains(t, strings.Join(strings.Fields(text), " "), "Milk 2 x 1.20 2.40")
}

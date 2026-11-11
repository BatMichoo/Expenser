package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

// minReceiptText is the least extracted text worth sending instead of the
// PDF; less means a scanned PDF (or just a page number).
const minReceiptText = 20

// pdfTextTimeout bounds one pdftotext run.
const pdfTextTimeout = 10 * time.Second

// pdfToText extracts a PDF's text with poppler's pdftotext, keeping the
// layout (quantity lines next to their items).
func pdfToText(ctx context.Context, pdf []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, pdfTextTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", "-", "-")
	cmd.Stdin = bytes.NewReader(pdf)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return "", errors.Join(err, errors.New(strings.TrimSpace(stderr.String())))
	}
	return out.String(), nil
}

// receiptParts is what Gemini gets for a receipt file. A PDF with text is
// sent as that text: Gemini reads PDF pages as pictures, and a long receipt
// printed on one tall page shrinks until its text is unreadable. Photos and
// scanned PDFs are sent as they are.
func receiptParts(ctx context.Context, data []byte, extract func(context.Context, []byte) (string, error)) []map[string]any {
	mimeType := http.DetectContentType(data)
	if mimeType == "application/pdf" {
		text, err := extract(ctx, data)
		switch {
		case err != nil:
			log.Printf("receipt PDF: no text extracted, sending the PDF: %v", err)
		case utf8.RuneCountInString(strings.TrimSpace(text)) >= minReceiptText:
			return []map[string]any{
				{"text": Prompts.GroceriesReceipt},
				{"text": "Receipt text:\n\n" + text},
			}
		}
	}
	return []map[string]any{
		{"text": Prompts.GroceriesReceipt},
		{"inline_data": map[string]any{
			"mime_type": mimeType,
			"data":      base64.StdEncoding.EncodeToString(data),
		}},
	}
}

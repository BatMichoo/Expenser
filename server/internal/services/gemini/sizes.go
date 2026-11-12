package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"expenser/internal/groceryunits"
)

// SizeSuggestion is the model's guess at what one counted quantity of a
// product is. The zero value means "no usable guess".
type SizeSuggestion struct {
	Unit     string  `json:"unit"`
	UnitSize float64 `json:"unit_size"`
}

// SuggestSizes guesses the unit and size of each product from its name
// alone (no receipt is sent). The result has one entry per product, in order;
// an entry that is not a storable size comes back as the zero value. An
// answer of the wrong length is a *ReplyError.
func (gs *GeminiService) SuggestSizes(ctx context.Context, products []string) ([]SizeSuggestion, error) {
	if gs.apiKey == "" || len(gs.models) == 0 {
		return nil, ErrNotConfigured
	}
	if len(products) == 0 {
		return nil, nil
	}
	names, err := json.Marshal(products)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]any{{"text": Prompts.SizeSuggestions}, {"text": string(names)}}},
		},
	})
	if err != nil {
		return nil, err
	}
	reply, finish, err := gs.generate(ctx, body)
	if err != nil {
		return nil, err
	}
	var out []SizeSuggestion
	if err := json.Unmarshal([]byte(reply), &out); err != nil {
		return nil, badReply(reply, finish, err)
	}
	if len(out) != len(products) {
		return nil, badReply(reply, finish, errors.New("suggestions do not match the products"))
	}
	for i, s := range out {
		size := groceryunits.Size{Unit: groceryunits.Unit(s.Unit), Per: s.UnitSize}
		if !groceryunits.Unit(s.Unit).Valid() || !size.Storable() {
			out[i] = SizeSuggestion{}
		}
	}
	return out, nil
}

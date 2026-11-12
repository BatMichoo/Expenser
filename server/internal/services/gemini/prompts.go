package gemini

// GeminiPrompts groups all prompt strings used when calling the Gemini API.
// This provides a centralized and organized way to reference prompts.
type GeminiPrompts struct {
	GroceriesReceipt string // instructs Gemini how to extract structured data from a receipt (image or text).
}

// Prompts is the main exported variable that provides access to all
// organized prompt strings. It should be imported and used
// globally for consistent prompt referencing.
var Prompts = &GeminiPrompts{
	GroceriesReceipt: `Extract the data from this grocery receipt and output it as a structured JSON object. The receipt is given either as an image or as its extracted text (with the printed layout kept).

	Quantity lines: some items have a line with the quantity and unit price (e.g. "2.000 x 1.78", "2 * 0,25" or "9,955 KG"). Receipt formats differ in where that line goes. In some (e.g. Lidl) it is printed just ABOVE its item and the item's total is on the item's own line; in others (e.g. Kaufland) it is printed just BELOW its item and carries the item's total. Work out which layout this receipt uses from the prices (quantity x unit price = line total) and pair each quantity line with the right item. Items without a quantity line have a quantity of 1 and a unit price equal to their line total. Decimal commas (4,48) are decimal points.

	Categorization instruction: For each item in "items", assign a category in the "category" field. The value MUST be one of the following exact string values, matching case and spelling:
	"Tomatoes", "Cucumbers", "Onions", "Garlic", "Peppers", "Carrots", "Lettuce", "Spinach", "Broccoli", "Mushrooms",
	"Milk", "Pork", "Beef", "Chicken", "Fish", "Sausages", "Bacon", "Ham", "Eggs", "Cheese", "Yogurt", "Butter",
	"Bread", "Rice", "Pasta", "Flour", "Sugar", "Cereal", "Beans", "Olive Oil", "Honey", "Ketchup",
	"Potatoes", "Apples", "Bananas", "Oranges", "Grapes", "Lemons", "Avocados",
	"Chips", "Nuts", "Chocolate", "Candy", "Cookies", "Ice Cream",
	"Water", "Juice", "Soda", "Coffee", "Tea", "Beer", "Wine", "Other".
	If an item does not clearly fit into the specific food items listed, assign "Other". Do not use any other category names.

	Units: for each item give "unit" and "unit_size", which say what ONE counted quantity of the item is. "unit" is "kg" (weight), "l" (volume) or "pc" (pieces). A weight line like "9,955 KG" means the item is sold by weight: unit "kg", unit_size 1. For packaged goods take the pack size from the product name: "1л" is unit "l", unit_size 1; "400г" is unit "kg", unit_size 0.4; "500мл" is unit "l", unit_size 0.5; "10бр" is unit "pc", unit_size 10. If the unit or size is unknown, give unit "" and unit_size 0. Do not guess.

	Items: list every item once, in the order printed on the receipt. A quantity line is part of its item, not an item of its own.

	Discounts: a discount printed directly with an item belongs in that item's "total_discount" (and "discount_per_unit"). A discount printed separately, usually after the subtotal with the items it applies to (e.g. "XTRA отстъпка -3,73" followed by "Позиции:19,15"), goes in "discounts" with its amount and "positions": the 1-based numbers of those items in the printed order (here [19, 15]); do NOT also put it on the items, it is applied to them later. Give amounts as positive numbers.

	Purchase date: put the date and time printed on the receipt (when the purchase was made) in "purchased_at" as "YYYY-MM-DDTHH:MM", 24-hour clock, exactly as printed (local time, no time zone). Bulgarian receipts usually print it as DD.MM.YYYY HH:MM:SS near the end, e.g. "12.11.2026 14:05:31" is "2026-11-12T14:05". If only the date is printed, give "YYYY-MM-DD". If there is no date or it cannot be read, give "". Do not guess and do not use today's date.

	Use the following JSON schema:
	{
	  "store": "string",
	  "address": "string",
	  "vat_number": "string",
	  "purchased_at": "string",
	  "items": [
	    {
	      "name": "string",
	      "quantity": number,
	      "unit_price": number,
		  "discount_per_unit": number,
	      "total_price": number,
		  "total_discount": number,
	      "category": "string",
	      "unit": "kg" | "l" | "pc" | "",
	      "unit_size": number
	    }
	  ],
	  "discounts": [
	    {
	      "description": "string",
	      "amount": number,
	      "positions": [number]
	    }
	  ],
	  "totals": {
	    "total_eur": number,
	    "total_bgn": number,
	    "exchange_rate": number
	  }
	}
	Return only the valid, parsed JSON object without any additional formatting or markdown blocks.
	`,
}

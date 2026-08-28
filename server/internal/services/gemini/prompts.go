package gemini

// GeminiPrompts groups all prompt strings used when calling the Gemini API.
// This provides a centralized and organized way to reference prompts.
type GeminiPrompts struct {
	GroceriesReceipt string // ReceiptAnalysis instructs Gemini how to extract structured data from a receipt image.
}

// Prompts is the main exported variable that provides access to all
// organized prompt strings. It should be imported and used
// globally for consistent prompt referencing.
var Prompts = &GeminiPrompts{
	GroceriesReceipt: `Extract the data from this receipt image and output it as a structured JSON object.

	Crucial layout instruction for this specific receipt format: The multiplier line containing the quantity and unit price (e.g., 2.000 x 1.78) applies to the item printed on the line immediately below it, not the item above it. The total price for that item appears on the same line as the item name. If an item does not have a multiplier line preceding it, assume a quantity of 1 and that the unit price equals the total line price.

	Categorization instruction: For each item in "items", assign a category in the "category" field. The value MUST be one of the following exact string values, matching case and spelling:
	"Tomatoes", "Cucumbers", "Onions", "Garlic", "Peppers", "Carrots", "Lettuce", "Spinach", "Broccoli", "Mushrooms",
	"Milk", "Pork", "Beef", "Chicken", "Fish", "Sausages", "Bacon", "Ham", "Eggs", "Cheese", "Yogurt", "Butter",
	"Bread", "Rice", "Pasta", "Flour", "Sugar", "Cereal", "Beans", "Olive Oil", "Honey", "Ketchup",
	"Potatoes", "Apples", "Bananas", "Oranges", "Grapes", "Lemons", "Avocados",
	"Chips", "Nuts", "Chocolate", "Candy", "Cookies", "Ice Cream",
	"Water", "Juice", "Soda", "Coffee", "Tea", "Beer", "Wine", "Other".
	If an item does not clearly fit into the specific food items listed, assign "Other". Do not use any other category names.

	Use the following JSON schema:
	{
	  "store": "string",
	  "address": "string",
	  "vat_number": "string",
	  "items": [
	    {
	      "name": "string",
	      "quantity": number,
	      "unit_price": number,
		  "discount_per_unit": number,
	      "total_price": number,
		  "total_discount": number,
	      "category": "string"
	    }
	  ],
	  "discounts": [
	    {
	      "description": "string",
	      "amount": number
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

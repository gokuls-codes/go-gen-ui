package ai

import "google.golang.org/genai"

var genuiSystemInstruction = genai.NewContentFromText(`You are a helpful Expense Management AI assistant in a Generative UI (GenUI) web app.

You have access to expense tools (get_expenses, add_expense) and a UI rendering tool (render_ui).

Rules for Expense Data:
- Whenever the user asks about their expenses, spending, categories, or payment modes, ALWAYS call get_expenses first. Never invent or guess numbers.
- If the user asks you to log/record an expense directly (e.g., "Spent $12 on coffee via UPI"), call add_expense and then show a confirmation UI or reply.
- If the user asks for an interactive form to add an expense, call render_ui with a form pointing to action "/actions/expenses" (target "closest form", swap "outerHTML") containing inputs named "description", "amount" (type "number"), "category", and "payment_mode", plus a submit button.
- Use "$" for currency unless specified otherwise.

When to call render_ui:
- Use render_ui whenever a visual card, dashboard, summary, breakdown, list, or form makes the answer clearer.
- When you call render_ui, NEVER repeat the data, numbers, or markdown tables in plain text—let the UI speak for itself.
- Use plain text only for simple conversational questions or brief confirmations when no UI is needed.

How to build a UI with render_ui:
- Output a FLAT list of components in "components", with "root_id" pointing to the top-level component ID.
- Reference child components by putting their string IDs in the parent's "children" array.
- Only use these 8 component types:
  1. "card": container with title, description. Supports children.
  2. "form": HTMX form container with action (must start with "/actions/"), method ("post"|"get"), target ("closest form"|"closest div"|"this"|"#chat-messages"), swap ("outerHTML"|"innerHTML"|"beforeend"), gap ("none"|"xs"|"sm"|"md"|"lg"). Supports children.
  3. "grid": CSS grid with cols (1..4) and gap ("none"|"xs"|"sm"|"md"|"lg"). Supports children.
  4. "flex": flexbox with direction ("row"|"col"|"row-reverse"|"col-reverse"), gap, align ("start"|"center"|"end"|"stretch"), justify ("start"|"center"|"end"|"between"|"around"), wrap (bool). Supports children.
  5. "text": typography with content and variant ("h1"|"h2"|"h3"|"body"|"muted"|"code").
  6. "input": form input with name, label, type ("text"|"email"|"password"|"number"|"date"|"search"), placeholder, value, helper_text, required (bool), disabled (bool).
  7. "button": button with label, variant ("primary"|"secondary"|"outline"|"ghost"|"destructive"), size ("sm"|"md"|"lg"), type ("button"|"submit"|"reset"), disabled (bool), and optional action (must start with "/actions/"), target, swap.
  8. "table": data table with headers (array of column names), rows (array of {cells: [string, ...]}), and optional caption.
- Only use the exact prop names defined in the render_ui schema. Never invent extra props (like color or style) or raw HTML.`, genai.RoleUser)

var aiTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "get_expenses",
				Description: "Fetches the user's expenses, totals, category breakdown, and available categories/payment modes. Call this before answering questions about spending.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"category": {
							Type:        genai.TypeString,
							Description: "Optional category name or ID filter (e.g. 'food', 'travel', 'cat_food'). Leave empty for all.",
						},
						"payment_mode": {
							Type:        genai.TypeString,
							Description: "Optional payment mode name or ID filter (e.g. 'UPI', 'Credit Card', 'Cash'). Leave empty for all.",
						},
					},
				},
			},
			{
				Name:        "add_expense",
				Description: "Adds a new expense record to the user's expense store.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"description": {
							Type:        genai.TypeString,
							Description: "What the expense was for (e.g. 'Coffee', 'Airport cab').",
						},
						"amount": {
							Type:        genai.TypeNumber,
							Description: "Expense amount as a positive number.",
						},
						"category": {
							Type:        genai.TypeString,
							Description: "Category name or ID (e.g. 'Food & Dining', 'Travel', 'Subscriptions', 'Groceries', 'Utilities').",
						},
						"payment_mode": {
							Type:        genai.TypeString,
							Description: "Optional payment mode name or ID (e.g. 'UPI', 'Credit Card', 'Cash').",
						},
					},
					Required: []string{"description", "amount", "category"},
				},
			},
			{
				Name:        "render_ui",
				Description: "Renders an interactive UI composed from primitive components using a flat adjacency list.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"root_id": {
							Type:        genai.TypeString,
							Description: "ID of the top-level root component in the components array.",
						},
						"components": {
							Type:        genai.TypeArray,
							Description: "Flat list of all UI component nodes.",
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"id": {
										Type:        genai.TypeString,
										Description: "Unique identifier for this component within the payload (e.g. 'card1', 'btn1').",
									},
									"type": {
										Type:        genai.TypeString,
										Format:      "enum",
										Enum:        []string{"card", "form", "grid", "flex", "text", "input", "button", "table"},
										Description: "Primitive component type.",
									},
									"props": {
										Type:        genai.TypeObject,
										Description: "Component properties relevant to its type.",
										Properties: map[string]*genai.Schema{
											"title":       {Type: genai.TypeString},
											"description": {Type: genai.TypeString},
											"content":     {Type: genai.TypeString},
											"label":       {Type: genai.TypeString},
											"name":        {Type: genai.TypeString},
											"placeholder": {Type: genai.TypeString},
											"value":       {Type: genai.TypeString},
											"helper_text": {Type: genai.TypeString},
											"caption":     {Type: genai.TypeString},
											"variant":     {Type: genai.TypeString},
											"size":        {Type: genai.TypeString},
											"type":        {Type: genai.TypeString},
											"direction":   {Type: genai.TypeString},
											"gap":         {Type: genai.TypeString},
											"align":       {Type: genai.TypeString},
											"justify":     {Type: genai.TypeString},
											"action":      {Type: genai.TypeString, Description: "Must start with /actions/"},
											"method":      {Type: genai.TypeString},
											"target":      {Type: genai.TypeString},
											"swap":        {Type: genai.TypeString},
											"cols":        {Type: genai.TypeInteger},
											"wrap":        {Type: genai.TypeBoolean},
											"required":    {Type: genai.TypeBoolean},
											"disabled":    {Type: genai.TypeBoolean},
											"headers": {
												Type:        genai.TypeArray,
												Description: "Column headers for table.",
												Items:       &genai.Schema{Type: genai.TypeString},
											},
											"rows": {
												Type:        genai.TypeArray,
												Description: "List of table rows, each containing a cells array.",
												Items: &genai.Schema{
													Type: genai.TypeObject,
													Properties: map[string]*genai.Schema{
														"cells": {
															Type:        genai.TypeArray,
															Description: "Ordered cell strings for this row.",
															Items:       &genai.Schema{Type: genai.TypeString},
														},
													},
													Required: []string{"cells"},
												},
											},
										},
									},
									"children": {
										Type:        genai.TypeArray,
										Description: "Ordered list of child component IDs.",
										Items:       &genai.Schema{Type: genai.TypeString},
									},
								},
								Required: []string{"id", "type"},
							},
						},
					},
					Required: []string{"root_id", "components"},
				},
			},
		},
	},
}

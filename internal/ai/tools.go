package ai

import "google.golang.org/genai"

var genuiSystemInstruction = genai.NewContentFromText(`You are a helpful AI assistant in a Generative UI (GenUI) web app.

You can respond with normal conversational text OR call the render_ui tool to generate interactive UI components.

When to call render_ui:
- Use render_ui whenever a visual card, form, layout, dashboard, or interactive action makes the answer clearer (e.g., feedback forms, calculators, summaries, comparisons, booking cards, settings).
- Use plain text for simple conversational questions or explanations.

How to build a UI with render_ui:
- Output a FLAT list of components in "components", with "root_id" pointing to the top-level component ID.
- Reference child components by putting their string IDs in the parent's "children" array.
- Only use these 7 component types:
  1. "card": container with title, description. Supports children.
  2. "form": HTMX form container with action (must start with "/actions/"), method ("post"|"get"), target ("closest form"|"closest div"|"this"|"#chat-messages"), swap ("outerHTML"|"innerHTML"|"beforeend"), gap ("none"|"xs"|"sm"|"md"|"lg"). Supports children.
  3. "grid": CSS grid with cols (1..4) and gap ("none"|"xs"|"sm"|"md"|"lg"). Supports children.
  4. "flex": flexbox with direction ("row"|"col"|"row-reverse"|"col-reverse"), gap, align ("start"|"center"|"end"|"stretch"), justify ("start"|"center"|"end"|"between"|"around"), wrap (bool). Supports children.
  5. "text": typography with content and variant ("h1"|"h2"|"h3"|"body"|"muted"|"code").
  6. "input": form input with name, label, type ("text"|"email"|"password"|"number"|"date"|"search"), placeholder, value, helper_text, required (bool), disabled (bool).
  7. "button": button with label, variant ("primary"|"secondary"|"outline"|"ghost"|"destructive"), size ("sm"|"md"|"lg"), type ("button"|"submit"|"reset"), disabled (bool), and optional action (must start with "/actions/"), target, swap.
- Never invent raw CSS classes or HTML.`, genai.RoleUser)

var renderUITool = &genai.Tool{
	FunctionDeclarations: []*genai.FunctionDeclaration{
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
									Enum:        []string{"card", "form", "grid", "flex", "text", "input", "button"},
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
}


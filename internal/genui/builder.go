package genui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/a-h/templ"
	"github.com/gokuls-codes/go-gen-ui/internal/views/components/primitives"
)

const maxTreeDepth = 32

// UINode represents a single primitive component in a flat adjacency-list payload.
type UINode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Props    map[string]any `json:"props,omitempty"`
	Children []string       `json:"children,omitempty"`
}

// UIPayload represents the flat component list emitted by the LLM.
type UIPayload struct {
	RootID     string   `json:"root_id"`
	Components []UINode `json:"components"`
}

func (n UINode) str(key string) string {
	if n.Props == nil {
		return ""
	}
	v, ok := n.Props[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func (n UINode) intVal(key string) int {
	if n.Props == nil {
		return 0
	}
	switch v := n.Props[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

func (n UINode) boolVal(key string) bool {
	if n.Props == nil {
		return false
	}
	b, _ := n.Props[key].(bool)
	return b
}

func (n UINode) strSlice(key string) []string {
	if n.Props == nil {
		return nil
	}
	return toStringSlice(n.Props[key])
}

func (n UINode) tableRows(key string) [][]string {
	if n.Props == nil {
		return nil
	}
	raw, ok := n.Props[key].([]any)
	if !ok {
		return nil
	}
	rows := make([][]string, 0, len(raw))
	for _, r := range raw {
		rowMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, toStringSlice(rowMap["cells"]))
	}
	return rows
}

func (n UINode) chartItems(key string) []primitives.ChartItem {
	if n.Props == nil {
		return nil
	}
	raw, ok := n.Props[key].([]any)
	if !ok {
		return nil
	}
	items := make([]primitives.ChartItem, 0, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		item := primitives.ChartItem{
			Label:   mapStr(m, "label"),
			Display: mapStr(m, "display"),
		}
		switch v := m["value"].(type) {
		case float64:
			item.Value = v
		case int:
			item.Value = float64(v)
		case int64:
			item.Value = float64(v)
		}
		items = append(items, item)
	}
	return items
}

func mapStr(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func toStringSlice(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if item == nil {
				out = append(out, "")
			} else if str, ok := item.(string); ok {
				out = append(out, str)
			} else {
				out = append(out, fmt.Sprint(item))
			}
		}
		return out
	default:
		return nil
	}
}

func (n UINode) resolvedID() string {
	if id := n.str("id"); id != "" {
		return id
	}
	return n.ID
}

// Build converts a flat UIPayload into a composed templ.Component tree.
func Build(payload UIPayload) (templ.Component, error) {
	if len(payload.Components) == 0 {
		return nil, errors.New("ui payload contains no components")
	}

	nodes := make(map[string]UINode, len(payload.Components))
	for _, node := range payload.Components {
		if node.ID != "" {
			nodes[node.ID] = node
		}
	}

	rootID := payload.RootID
	if rootID == "" {
		rootID = payload.Components[0].ID
	}

	comp := buildNode(rootID, nodes, make(map[string]bool), 0)
	if comp == nil {
		return nil, fmt.Errorf("root component %q not found or invalid", rootID)
	}
	return comp, nil
}

func buildNode(id string, nodes map[string]UINode, visiting map[string]bool, depth int) templ.Component {
	if depth >= maxTreeDepth || visiting[id] {
		return nil
	}
	node, ok := nodes[id]
	if !ok {
		return nil
	}

	comp := mapPrimitive(node)
	if comp == nil {
		return nil
	}

	if len(node.Children) == 0 {
		return comp
	}

	visiting[id] = true
	defer delete(visiting, id)

	var childComponents []templ.Component
	for _, childID := range node.Children {
		if child := buildNode(childID, nodes, visiting, depth+1); child != nil {
			childComponents = append(childComponents, child)
		}
	}

	if len(childComponents) == 0 {
		return comp
	}

	joined := templ.Join(childComponents...)
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return comp.Render(templ.WithChildren(ctx, joined), w)
	})
}

func safeAction(action string) string {
	action = strings.TrimSpace(action)
	if action == "" || strings.Contains(action, "..") || strings.Contains(action, "//") {
		return ""
	}
	if action == "/actions/submit" || strings.HasPrefix(action, "/actions/submit?") {
		return action
	}
	return ""
}

func safeTarget(target string) string {
	switch target {
	case "this", "closest form", "closest div", "#chat-messages":
		return target
	default:
		return ""
	}
}

func safeSwap(swap string) string {
	switch swap {
	case "outerHTML", "innerHTML", "beforeend", "afterend":
		return swap
	default:
		return ""
	}
}

func mapPrimitive(node UINode) templ.Component {
	switch node.Type {
	case "button":
		return primitives.Button(primitives.ButtonProps{
			ID:       node.resolvedID(),
			Label:    node.str("label"),
			Variant:  node.str("variant"),
			Size:     node.str("size"),
			Type:     node.str("type"),
			Disabled: node.boolVal("disabled"),
			Action:   safeAction(node.str("action")),
			Target:   safeTarget(node.str("target")),
			Swap:     safeSwap(node.str("swap")),
		})
	case "card":
		return primitives.Card(primitives.CardProps{
			ID:          node.resolvedID(),
			Title:       node.str("title"),
			Description: node.str("description"),
		})
	case "chart":
		return primitives.Chart(primitives.ChartProps{
			Title:     node.str("title"),
			ChartType: node.str("chart_type"),
			Items:     node.chartItems("items"),
		})
	case "flex":
		return primitives.Flex(primitives.FlexProps{
			Direction: node.str("direction"),
			Gap:       node.str("gap"),
			Align:     node.str("align"),
			Justify:   node.str("justify"),
			Wrap:      node.boolVal("wrap"),
		})
	case "form":
		action := safeAction(node.str("action"))
		if action == "" {
			action = "/actions/submit"
		}
		target := safeTarget(node.str("target"))
		if target == "" {
			target = "closest form"
		}
		swap := safeSwap(node.str("swap"))
		if swap == "" {
			swap = "outerHTML"
		}
		return primitives.Form(primitives.FormProps{
			ID:     node.resolvedID(),
			Action: action,
			Method: node.str("method"),
			Target: target,
			Swap:   swap,
			Gap:    node.str("gap"),
		})
	case "grid":
		return primitives.Grid(primitives.GridProps{
			Cols: node.intVal("cols"),
			Gap:  node.str("gap"),
		})
	case "input":
		return primitives.Input(primitives.InputProps{
			ID:           node.resolvedID(),
			Name:         node.str("name"),
			Label:        node.str("label"),
			Type:         node.str("type"),
			Placeholder:  node.str("placeholder"),
			Value:        node.str("value"),
			HelperText:   node.str("helper_text"),
			Error:        node.str("error"),
			Required:     node.boolVal("required"),
			Disabled:     node.boolVal("disabled"),
			Autocomplete: node.str("autocomplete"),
		})
	case "select":
		return primitives.Select(primitives.SelectProps{
			ID:          node.resolvedID(),
			Name:        node.str("name"),
			Label:       node.str("label"),
			Options:     node.strSlice("options"),
			Value:       node.str("value"),
			Placeholder: node.str("placeholder"),
			HelperText:  node.str("helper_text"),
			Error:       node.str("error"),
			Required:    node.boolVal("required"),
			Disabled:    node.boolVal("disabled"),
		})
	case "table":
		return primitives.Table(primitives.TableProps{
			Headers: node.strSlice("headers"),
			Rows:    node.tableRows("rows"),
			Caption: node.str("caption"),
		})
	case "text":
		return primitives.Text(primitives.TextProps{
			Content: node.str("content"),
			Variant: node.str("variant"),
		})
	default:
		return nil
	}
}

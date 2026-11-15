// Package ui defines declarative UI contribution documents interpreted by Fleet (OPEN-07).
package ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const SchemaVersion = 1

var routePattern = regexp.MustCompile(`^/ext/[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._/-]*$`)

// Contribution is the declarative UI entry loaded from frontend.entry.
type Contribution struct {
	Schema      int      `json:"schema"`
	ExtensionID string   `json:"extensionId"`
	Columns     []Column `json:"columns,omitempty"`
	Actions     []Action `json:"actions,omitempty"`
	Pages       []Page   `json:"pages,omitempty"`
	Panels      []Panel  `json:"panels,omitempty"`
	Forms       []Form   `json:"forms,omitempty"`
}

type Column struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Field string `json:"field"`
	Width int    `json:"width,omitempty"`
}

type Action struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Target  string `json:"target"`
	Confirm bool   `json:"confirm,omitempty"`
	Primary bool   `json:"primary,omitempty"`
	Danger  bool   `json:"danger,omitempty"`
}

type Page struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Route string `json:"route"`
}

type Panel struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Placement string `json:"placement"`
}

type Form struct {
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	Description  string      `json:"description,omitempty"`
	SubmitLabel  string      `json:"submitLabel,omitempty"`
	LoadMethod   string      `json:"loadMethod,omitempty"`
	SubmitMethod string      `json:"submitMethod,omitempty"`
	Fields       []FormField `json:"fields"`
}

type FormFieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type FormField struct {
	Name        string            `json:"name"`
	Label       string            `json:"label"`
	Kind        string            `json:"kind"`
	Required    bool              `json:"required,omitempty"`
	Help        string            `json:"help,omitempty"`
	Placeholder string            `json:"placeholder,omitempty"`
	Advanced    bool              `json:"advanced,omitempty"`
	Rows        int               `json:"rows,omitempty"`
	Options     []FormFieldOption `json:"options,omitempty"`
}

// ParseJSON unmarshals and validates a contribution document.
func ParseJSON(data []byte) (*Contribution, error) {
	var c Contribution
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse ui contribution: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate enforces the declarative UI contract.
func (c *Contribution) Validate() error {
	if c == nil {
		return fmt.Errorf("contribution is nil")
	}
	if c.Schema != SchemaVersion {
		return fmt.Errorf("unsupported schema version %d", c.Schema)
	}
	if strings.TrimSpace(c.ExtensionID) == "" {
		return fmt.Errorf("extensionId required")
	}
	for i, col := range c.Columns {
		if col.ID == "" || col.Label == "" || col.Field == "" {
			return fmt.Errorf("columns[%d]: id, label, field required", i)
		}
		if col.Width != 0 && (col.Width < 40 || col.Width > 800) {
			return fmt.Errorf("columns[%d]: invalid width", i)
		}
	}
	for i, a := range c.Actions {
		if a.ID == "" || a.Label == "" {
			return fmt.Errorf("actions[%d]: id and label required", i)
		}
		switch a.Target {
		case "torrent", "selection", "global":
		default:
			return fmt.Errorf("actions[%d]: invalid target %q", i, a.Target)
		}
	}
	for i, p := range c.Pages {
		if p.ID == "" || p.Title == "" || !routePattern.MatchString(p.Route) {
			return fmt.Errorf("pages[%d]: invalid page", i)
		}
	}
	for i, p := range c.Panels {
		if p.ID == "" || p.Title == "" {
			return fmt.Errorf("panels[%d]: id and title required", i)
		}
		switch p.Placement {
		case "torrent-details", "sidebar", "settings":
		default:
			return fmt.Errorf("panels[%d]: invalid placement %q", i, p.Placement)
		}
	}
	for i, f := range c.Forms {
		if f.ID == "" || f.Title == "" || len(f.Fields) == 0 {
			return fmt.Errorf("forms[%d]: id, title, fields required", i)
		}
		for j, field := range f.Fields {
			if field.Name == "" || field.Label == "" {
				return fmt.Errorf("forms[%d].fields[%d]: name and label required", i, j)
			}
			switch field.Kind {
			case "text", "number", "boolean", "select", "url", "textarea":
			default:
				return fmt.Errorf("forms[%d].fields[%d]: invalid kind %q", i, j, field.Kind)
			}
			if field.Kind == "select" && len(field.Options) == 0 {
				return fmt.Errorf("forms[%d].fields[%d]: select requires options", i, j)
			}
			if field.Rows != 0 && (field.Rows < 2 || field.Rows > 40) {
				return fmt.Errorf("forms[%d].fields[%d]: invalid rows", i, j)
			}
			for k, opt := range field.Options {
				if opt.Label == "" {
					return fmt.Errorf("forms[%d].fields[%d].options[%d]: label required", i, j, k)
				}
			}
		}
	}
	return nil
}

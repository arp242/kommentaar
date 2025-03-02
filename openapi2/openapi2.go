// Package openapi2 outputs to OpenAPI 2.0
//
// https://github.com/OAI/OpenAPI-Specification/blob/master/versions/2.0.md
// http://json-schema.org/
package openapi2

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"zgo.at/kommentaar/docparse"
	"zgo.at/kommentaar/zgo"
)

type (
	// OpenAPI output.
	OpenAPI struct {
		Swagger string `json:"swagger"`
		Info    Info   `json:"info"`

		// TODO: hacked on :-/
		SecurityDefinitions map[string]any   `json:"securityDefinitions,omitempty"`
		Security            []map[string]any `json:"security,omitempty"`

		// TODO: do we need this? will have to come from config
		Host     string   `json:"host,omitempty"`
		BasePath string   `json:"basePath,omitempty"`
		Schemes  []string `json:"schemes,omitempty"`
		Consumes []string `json:"consumes,omitempty"`
		Produces []string `json:"produces,omitempty"`

		Tags        []Tag                      `json:"tags,omitempty"`
		Paths       map[string]*Path           `json:"paths"`
		Definitions map[string]docparse.Schema `json:"definitions"`
	}

	// Info provides metadata about the API.
	Info struct {
		Title       string  `json:"title,omitempty"`
		Description string  `json:"description,omitempty"`
		Version     string  `json:"version,omitempty"`
		Contact     Contact `json:"contact,omitempty"`
	}

	// Contact provides contact information for the exposed API.
	Contact struct {
		Name  string `json:"name,omitempty"`
		URL   string `json:"url,omitempty"`
		Email string `json:"email,omitempty"`
	}

	// Parameter describes a single operation parameter.
	Parameter struct {
		Name        string           `json:"name"`
		In          string           `json:"in"` // query, header, path, cookie
		Description string           `json:"description,omitempty"`
		Type        string           `json:"type,omitempty"`
		Items       *docparse.Schema `json:"items,omitempty"`
		Format      string           `json:"format,omitempty"`
		Required    bool             `json:"required,omitempty"`
		Readonly    *bool            `json:"readOnly,omitempty"`
		Enum        []string         `json:"enum,omitempty"`
		Default     string           `json:"default,omitempty"`
		Minimum     int              `json:"minimum,omitempty"`
		Maximum     int              `json:"maximum,omitempty"`
		Schema      *docparse.Schema `json:"schema,omitempty"`
	}

	// Tag adds metadata to a single tag that is used by the Operation type.
	Tag struct {
		Name string `json:"name"`
	}

	// Path describes the operations available on a single path.
	Path struct {
		Ref    string     `json:"ref,omitempty"`
		Get    *Operation `json:"get,omitempty"`
		Post   *Operation `json:"post,omitempty"`
		Put    *Operation `json:"put,omitempty"`
		Patch  *Operation `json:"patch,omitempty"`
		Delete *Operation `json:"delete,omitempty"`
		Head   *Operation `json:"head,omitempty"`
	}

	// Operation describes a single API operation on a path.
	Operation struct {
		OperationID string           `json:"operationId"`
		Tags        []string         `json:"tags,omitempty"`
		Summary     string           `json:"summary,omitempty"`
		Description string           `json:"description,omitempty"`
		Consumes    []string         `json:"consumes,omitempty"`
		Produces    []string         `json:"produces,omitempty"`
		Parameters  []Parameter      `json:"parameters,omitempty"`
		Responses   map[int]Response `json:"responses"`
	}

	// Reference other components in the specification, internally and
	// externally.
	Reference struct {
		Ref string `json:"$ref"`
	}

	// Response describes a single response from an API Operation.
	Response struct {
		Description string           `json:"description,omitempty"`
		Schema      *docparse.Schema `json:"schema,omitempty"`
	}
)

func (o *Operation) toMap() (map[string]any, error) {
	type Alias Operation
	data, err := json.Marshal((*Alias)(o))
	if err != nil {
		return nil, fmt.Errorf("json marshal: %v", err)
	}

	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("json unmarshal: %v", err)
	}

	return m, nil
}

// MarshalJSON implements the json.Marshaler interface.
func (o *Operation) MarshalJSON() ([]byte, error) {
	m, err := o.toMap()
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// WriteJSON writes to w as JSON.
func WriteJSON(w io.Writer, prog *docparse.Program) error {
	return write("json", w, prog)
}

// WriteJSONIndent writes to w as indented JSON.
func WriteJSONIndent(w io.Writer, prog *docparse.Program) error {
	return write("jsonindent", w, prog)
}

func write(outFormat string, w io.Writer, prog *docparse.Program) error {
	out := OpenAPI{
		Swagger:  "2.0",
		BasePath: prog.Config.Basepath,
		Info: Info{
			Title:       prog.Config.Title,
			Description: string(prog.Config.Description),
			Version:     prog.Config.Version,
			Contact: Contact{
				Name:  prog.Config.ContactName,
				Email: prog.Config.ContactEmail,
				URL:   prog.Config.ContactSite,
			},
		},
		Consumes:    []string{prog.Config.DefaultRequestCt},
		Produces:    []string{prog.Config.DefaultRequestCt},
		Paths:       map[string]*Path{},
		Definitions: map[string]docparse.Schema{},
	}

	// Auth info
	switch prog.Config.Auth {
	default:
		return fmt.Errorf("unknown auth value: %q", prog.Config.Auth)
	case "basic":
		out.SecurityDefinitions = map[string]any{
			"basicAuth": map[string]any{"type": "basic"},
		}
		out.Security = []map[string]any{
			{"basicAuth": []string{}},
		}
	}

	// Add definitions.
	for k, v := range prog.References {
		if v.Schema == nil {
			return fmt.Errorf("schema is nil for %q", k)
		}
		switch v.Context {
		case "form", "query", "path":
			// Nothing, this will be inline in the operation.
		default:
			if !v.IsEmbed {
				prefixPropertyReferences(v.Schema.Properties)
				out.Definitions[k] = *v.Schema
			}
		}
	}

	seenTags := map[string]struct{}{}

	// Add endpoints.
	for _, e := range prog.Endpoints {
		e.Path = prog.Config.Prefix + e.Path

		op := Operation{
			Summary:     e.Tagline,
			Description: e.Info,
			OperationID: makeID(e),
			Tags:        e.Tags,
			Responses:   map[int]Response{},
		}

		// Add their tags to the top level object to ensure ordering in
		// various tools:
		for _, t := range e.Tags {
			seenTags[t] = struct{}{}
		}

		// Add path params.
		if e.Request.Path != nil {
			// TODO: Don't access prog.References directly. This probably
			// shouldn't be there anyway.
			ref := prog.References[e.Request.Path.Reference]

			for name, p := range ref.Schema.Properties {
				if p.OmitDoc {
					// path is required, so just blank description.
					p.Description = ""
				}

				op.Parameters = append(op.Parameters, Parameter{
					Name:        name,
					In:          "path",
					Description: p.Description,
					Type:        p.Type,
					Required:    true,
				})
			}
		}

		// Add query params.
		if e.Request.Query != nil {
			// TODO: Don't access prog.References directly. This probably
			// shouldn't be there anyway.
			ref := prog.References[e.Request.Query.Reference]

			for _, f := range ref.Fields {
				// TODO: this should be done in docparse.
				f.Name = zgo.TagName(f.KindField, "query")
				if f.Name == "-" {
					continue
				}

				schema := ref.Schema.Properties[f.Name]
				if schema == nil {
					return fmt.Errorf("schema is nil for query field %q in %q",
						f.Name, e.Request.Query.Reference)
				}
				if schema.OmitDoc {
					continue
				}

				queryType := schema.Type
				if len(queryType) == 0 {
					// if the parameter is a struct, and not mapped,
					// we should fallback to a string to have a valid swagger file
					// (we can not have a field without schema nor type )
					queryType = "string"
				}

				items := schema.Items
				if items != nil && len(items.Reference) != 0 {
					// in swagger 2.0, arrays in the query can only
					// contain basic type, so, if it holds a reference
					// we change it to a string
					items = &docparse.Schema{
						Type: "string",
					}
				}

				op.Parameters = append(op.Parameters, Parameter{
					Name:        f.Name,
					In:          "query",
					Description: schema.Description,
					Type:        queryType,
					Items:       items,
					Required:    len(schema.Required) > 0,
					Readonly:    schema.Readonly,
					Enum:        schema.Enum,
					Default:     schema.Default,
					Minimum:     schema.Minimum,
					Maximum:     schema.Maximum,
					Format:      schema.Format,
				})
			}
		}

		// Add form params,
		if e.Request.Form != nil {
			// TODO: Don't access prog.References directly. This probably
			// shouldn't be there anyway.
			ref := prog.References[e.Request.Form.Reference]

			for _, f := range ref.Fields {
				// TODO: this should be done in docparse
				f.Name = zgo.TagName(f.KindField, "form")
				if f.Name == "-" {
					continue
				}

				schema := ref.Schema.Properties[f.Name]
				if schema == nil {
					return fmt.Errorf("schema is nil for form field %q in %q",
						f.Name, e.Request.Query.Reference)
				}
				if schema.OmitDoc {
					continue
				}

				formType := schema.Type
				if len(formType) == 0 {
					// if the parameter is a struct, and not mapped,
					// we should fallback to a string to have a valid swagger file
					// (we can not have a field without schema nor type )
					formType = "string"
				}
				op.Parameters = append(op.Parameters, Parameter{
					Name:        f.Name,
					In:          "formData",
					Description: schema.Description,
					Type:        formType,
					Items:       schema.Items,
					Required:    len(schema.Required) > 0,
					Readonly:    schema.Readonly,
					Enum:        schema.Enum,
					Default:     schema.Default,
					Minimum:     schema.Minimum,
					Maximum:     schema.Maximum,
					Format:      schema.Format,
				})
			}
			op.Consumes = append(op.Consumes, "application/x-www-form-urlencoded")
		}

		// Add any {..} parameters in the path to the parameter list if they
		// haven't been specified manually in e.Request.Path.
		if strings.Contains(e.Path, "{") && e.Request.Path == nil {
			for _, param := range docparse.PathParams(e.Path) {
				// TODO: allow setting this type; this is a bit of a hack
				t := "string"
				if p := strings.ToLower(param); p == "id" || strings.HasSuffix(p, "_id") {
					t = "integer"
				}
				param = strings.Trim(param, "{}")
				op.Parameters = append(op.Parameters, Parameter{
					Name:     param,
					In:       "path",
					Type:     t,
					Required: true,
				})
			}
		}

		if e.Request.Body != nil {
			op.Parameters = append(op.Parameters, Parameter{
				// TODO: name required, is there a better value to use?
				Name:        e.Request.Body.Reference,
				In:          "body",
				Description: e.Request.Body.Description,
				Required:    true,
				Schema: &docparse.Schema{
					Reference: "#/definitions/" + e.Request.Body.Reference,
				},
			})
			op.Consumes = append(op.Consumes, e.Request.ContentType)
		}

		// TODO: preserve order in which they were defined in the struct, but
		// for now sort it like this so the output is stable.
		sort.Slice(op.Parameters, func(i, j int) bool {
			return op.Parameters[i].Type+op.Parameters[i].Name > op.Parameters[j].Type+op.Parameters[j].Name
		})

		for code, resp := range e.Responses {
			r := Response{
				Description: resp.Body.Description,
			}

			// Link reference.
			if resp.Body != nil && resp.Body.Reference != "" {
				r.Schema = &docparse.Schema{
					Reference: "#/definitions/" + resp.Body.Reference,
				}
			} else if dr, ok := prog.Config.DefaultResponse[code]; ok {
				r.Schema = &docparse.Schema{
					Reference: "#/definitions/" + dr.Body.Reference,
				}
				if dr.ContentType != "" {
					resp.ContentType = dr.ContentType
				}
			}

			op.Responses[code] = r
			op.Produces = appendIfNotExists(op.Produces, resp.ContentType)
		}

		sort.Strings(op.Produces)

		if out.Paths[e.Path] == nil {
			out.Paths[e.Path] = &Path{}
		}

		switch e.Method {
		case "GET":
			out.Paths[e.Path].Get = &op
		case "POST":
			out.Paths[e.Path].Post = &op
		case "PUT":
			out.Paths[e.Path].Put = &op
		case "PATCH":
			out.Paths[e.Path].Patch = &op
		case "DELETE":
			out.Paths[e.Path].Delete = &op
		case "HEAD":
			out.Paths[e.Path].Head = &op
		default:
			return fmt.Errorf("unknown method: %#v", e.Method)
		}
	}

	if len(seenTags) > 0 {
		out.Tags = make([]Tag, 0, len(seenTags))
		for tag := range seenTags {
			out.Tags = append(out.Tags, Tag{Name: tag})
		}
		sort.Slice(out.Tags, func(i int, j int) bool {
			return out.Tags[i].Name < out.Tags[j].Name
		})
	}

	var (
		d   []byte
		err error
	)
	switch outFormat {
	case "jsonindent":
		d, err = json.MarshalIndent(&out, "", "  ")
	case "json":
		d, err = json.Marshal(&out)
	default:
		err = fmt.Errorf("unknown format: %#v", outFormat)
	}
	if err != nil {
		return err
	}

	_, err = w.Write(d)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

func makeID(e *docparse.Endpoint) string {
	return strings.Replace(fmt.Sprintf("%v_%v", e.Method,
		strings.Replace(e.Path, "/", "_", -1)), "__", "_", 1)
}

func appendIfNotExists(xs []string, y string) []string {
	for _, x := range xs {
		if x == y {
			return xs
		}
	}
	return append(xs, y)
}

func prefixPropertyReferences(properties map[string]*docparse.Schema) {
	var rm []string
	for k, s := range properties {
		if s.Reference != "" && !strings.HasPrefix(s.Reference, "#/definitions/") {
			s.Reference = "#/definitions/" + s.Reference
		}
		if s.Items != nil && s.Items.Reference != "" {
			if !strings.HasPrefix(s.Items.Reference, "#/definitions/") {
				s.Items.Reference = "#/definitions/" + s.Items.Reference
			}
		}
		if s.AdditionalProperties != nil && s.AdditionalProperties.Reference != "" {
			if !strings.HasPrefix(s.AdditionalProperties.Reference, "#/definitions/") {
				s.AdditionalProperties.Reference = "#/definitions/" + s.AdditionalProperties.Reference
			}
		}

		if s.OmitDoc {
			rm = append(rm, k)
		}

		if s.Properties != nil {
			prefixPropertyReferences(s.Properties)
		}
	}

	for _, r := range rm {
		delete(properties, r)
	}
}

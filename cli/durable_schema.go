package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type DurableSurfaceSchemaFormat string

const (
	DurableSurfaceSchemaFormatJSON     DurableSurfaceSchemaFormat = "json"
	DurableSurfaceSchemaFormatJSONL    DurableSurfaceSchemaFormat = "jsonl"
	DurableSurfaceSchemaFormatMarkdown DurableSurfaceSchemaFormat = "markdown"
)

type DurableSurfaceSchemaSpec struct {
	AuthoringFormat     DurableSurfaceSchemaFormat
	StorageFormat       DurableSurfaceSchemaFormat
	Summary             string
	Example             string
	FieldNotes          []string
	FrameworkOwnedFields []string
	AllowedKinds        []string
}

type DurableSurfaceSchema struct {
	Surface              DurableSurfaceName
	Class                DurableSurfaceClass
	WriteMode            DurableSurfaceWriteMode
	Strict               bool
	FrameworkReadsBody   bool
	AuthoringFormat      DurableSurfaceSchemaFormat
	StorageFormat        DurableSurfaceSchemaFormat
	Summary              string
	Example              string
	FieldNotes           []string
	FrameworkOwnedFields []string
	AllowedKinds         []string
}

type DurableContract struct {
	Surface              DurableSurfaceName         `json:"surface"`
	Class                DurableSurfaceClass        `json:"class"`
	WriteMode            DurableSurfaceWriteMode    `json:"write_mode"`
	Strict               bool                       `json:"strict"`
	FrameworkReadsBody   bool                       `json:"framework_reads_body,omitempty"`
	AuthoringFormat      DurableSurfaceSchemaFormat `json:"authoring_format"`
	StorageFormat        DurableSurfaceSchemaFormat `json:"storage_format,omitempty"`
	Summary              string                     `json:"summary,omitempty"`
	Example              string                     `json:"example,omitempty"`
	Notes                []string                   `json:"notes,omitempty"`
	FrameworkOwnedFields []string                   `json:"framework_owned_fields,omitempty"`
	AllowedKinds         []string                   `json:"allowed_kinds,omitempty"`
}

func LookupDurableSurfaceSchema(name string) (DurableSurfaceSchema, error) {
	spec, err := LookupDurableSurface(name)
	if err != nil {
		return DurableSurfaceSchema{}, err
	}
	if strings.TrimSpace(spec.Schema.Summary) == "" {
		return DurableSurfaceSchema{}, fmt.Errorf("durable surface %q is missing schema summary", spec.Name)
	}
	if strings.TrimSpace(spec.Schema.Example) == "" {
		return DurableSurfaceSchema{}, fmt.Errorf("durable surface %q is missing schema example", spec.Name)
	}
	if strings.TrimSpace(string(spec.Schema.AuthoringFormat)) == "" {
		return DurableSurfaceSchema{}, fmt.Errorf("durable surface %q is missing authoring schema format", spec.Name)
	}
	if strings.TrimSpace(string(spec.Schema.StorageFormat)) == "" {
		return DurableSurfaceSchema{}, fmt.Errorf("durable surface %q is missing storage schema format", spec.Name)
	}
	return DurableSurfaceSchema{
		Surface:              spec.Name,
		Class:                spec.Class,
		WriteMode:            spec.WriteMode,
		Strict:               spec.Strict,
		FrameworkReadsBody:   spec.FrameworkReadsBody,
		AuthoringFormat:      spec.Schema.AuthoringFormat,
		StorageFormat:        spec.Schema.StorageFormat,
		Summary:              spec.Schema.Summary,
		Example:              spec.Schema.Example,
		FieldNotes:           append([]string(nil), spec.Schema.FieldNotes...),
		FrameworkOwnedFields: append([]string(nil), spec.Schema.FrameworkOwnedFields...),
		AllowedKinds:         append([]string(nil), spec.Schema.AllowedKinds...),
	}, nil
}

func LookupDurableContract(name string) (DurableContract, error) {
	schema, err := LookupDurableSurfaceSchema(name)
	if err != nil {
		return DurableContract{}, err
	}
	return DurableContract{
		Surface:              schema.Surface,
		Class:                schema.Class,
		WriteMode:            schema.WriteMode,
		Strict:               schema.Strict,
		FrameworkReadsBody:   schema.FrameworkReadsBody,
		AuthoringFormat:      schema.AuthoringFormat,
		StorageFormat:        schema.StorageFormat,
		Summary:              schema.Summary,
		Example:              prettyDurableSchemaExample(schema.AuthoringFormat, schema.Example),
		Notes:                append([]string(nil), schema.FieldNotes...),
		FrameworkOwnedFields: append([]string(nil), schema.FrameworkOwnedFields...),
		AllowedKinds:         append([]string(nil), schema.AllowedKinds...),
	}, nil
}

func RenderDurableContract(name string) (string, error) {
	contract, err := LookupDurableContract(name)
	if err != nil {
		return "", err
	}
	return renderDurableContract(contract), nil
}

func renderDurableContract(contract DurableContract) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# GoalX Schema: %s\n\n", contract.Surface))
	b.WriteString(fmt.Sprintf("- Surface: `%s`\n", contract.Surface))
	b.WriteString(fmt.Sprintf("- Class: `%s`\n", contract.Class))
	b.WriteString(fmt.Sprintf("- Write mode: `%s`\n", contract.WriteMode))
	b.WriteString(fmt.Sprintf("- Authoring format: `%s`\n", contract.AuthoringFormat))
	b.WriteString(fmt.Sprintf("- Storage format: `%s`\n", contract.StorageFormat))
	b.WriteString(fmt.Sprintf("- Strict: `%t`\n", contract.Strict))
	b.WriteString(fmt.Sprintf("- Framework reads body: `%t`\n", contract.FrameworkReadsBody))
	if strings.TrimSpace(contract.Summary) != "" {
		b.WriteString("\n")
		b.WriteString(contract.Summary)
		b.WriteString("\n")
	}
	if cmd := durableSchemaWriteCommand(contract); cmd != "" {
		b.WriteString("\nWrite path:\n")
		b.WriteString(fmt.Sprintf("`%s`\n", cmd))
	}
	notes := make([]string, 0, len(contract.Notes)+1)
	if contract.Strict {
		notes = append(notes, "Unknown fields are fatal.")
	}
	notes = append(notes, contract.Notes...)
	if len(notes) > 0 {
		b.WriteString("\nNotes:\n")
		for _, note := range notes {
			if strings.TrimSpace(note) == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(note)
			b.WriteString("\n")
		}
	}
	if len(contract.FrameworkOwnedFields) > 0 {
		b.WriteString("\nFramework-owned fields:\n")
		for _, field := range contract.FrameworkOwnedFields {
			if strings.TrimSpace(field) == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(field)
			b.WriteString("\n")
		}
	}
	if len(contract.AllowedKinds) > 0 {
		b.WriteString("\nAllowed kinds:\n")
		for _, kind := range contract.AllowedKinds {
			if strings.TrimSpace(kind) == "" {
				continue
			}
			b.WriteString("- `")
			b.WriteString(kind)
			b.WriteString("`\n")
		}
	}
	if strings.TrimSpace(contract.Example) != "" {
		b.WriteString("\nExample:\n")
		b.WriteString("```")
		b.WriteString(string(contract.AuthoringFormat))
		b.WriteString("\n")
		b.WriteString(contract.Example)
		if !strings.HasSuffix(contract.Example, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n")
	}
	return b.String()
}

func prettyDurableSchemaExample(format DurableSurfaceSchemaFormat, example string) string {
	example = strings.TrimSpace(example)
	if example == "" {
		return ""
	}
	switch format {
	case DurableSurfaceSchemaFormatJSON:
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(example), "", "  "); err == nil {
			return pretty.String()
		}
	case DurableSurfaceSchemaFormatJSONL:
		lines := strings.Split(example, "\n")
		formattedLines := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, []byte(line), "", "  "); err == nil {
				formattedLines = append(formattedLines, pretty.String())
				continue
			}
			formattedLines = append(formattedLines, line)
		}
		if len(formattedLines) > 0 {
			return strings.Join(formattedLines, "\n")
		}
	}
	return example
}

func durableSchemaHintError(surface DurableSurfaceName, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w; see `goalx schema %s`", err, surface)
}

func durableSchemaWriteCommand(contract DurableContract) string {
	switch contract.Class {
	case DurableSurfaceClassStructuredState:
		return fmt.Sprintf("goalx durable write %s --run NAME --body-file /abs/path.json", contract.Surface)
	case DurableSurfaceClassEventLog:
		kind := "KIND"
		if len(contract.AllowedKinds) > 0 {
			kind = contract.AllowedKinds[0]
		}
		return fmt.Sprintf("goalx durable write %s --run NAME --kind %s --actor master --body-file /abs/path.json", contract.Surface, kind)
	}
	return ""
}

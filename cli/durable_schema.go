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
	Format     DurableSurfaceSchemaFormat
	Summary    string
	Example    string
	FieldNotes []string
}

type DurableSurfaceSchema struct {
	Surface            DurableSurfaceName
	Class              DurableSurfaceClass
	WriteMode          DurableSurfaceWriteMode
	Strict             bool
	FrameworkReadsBody bool
	Format             DurableSurfaceSchemaFormat
	Summary            string
	Example            string
	FieldNotes         []string
}

type DurableContract struct {
	Surface            DurableSurfaceName         `json:"surface"`
	Class              DurableSurfaceClass        `json:"class"`
	WriteMode          DurableSurfaceWriteMode    `json:"write_mode"`
	Strict             bool                       `json:"strict"`
	FrameworkReadsBody bool                       `json:"framework_reads_body,omitempty"`
	Format             DurableSurfaceSchemaFormat `json:"format"`
	Summary            string                     `json:"summary,omitempty"`
	Example            string                     `json:"example,omitempty"`
	Notes              []string                   `json:"notes,omitempty"`
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
	if strings.TrimSpace(string(spec.Schema.Format)) == "" {
		return DurableSurfaceSchema{}, fmt.Errorf("durable surface %q is missing schema format", spec.Name)
	}
	return DurableSurfaceSchema{
		Surface:            spec.Name,
		Class:              spec.Class,
		WriteMode:          spec.WriteMode,
		Strict:             spec.Strict,
		FrameworkReadsBody: spec.FrameworkReadsBody,
		Format:             spec.Schema.Format,
		Summary:            spec.Schema.Summary,
		Example:            spec.Schema.Example,
		FieldNotes:         append([]string(nil), spec.Schema.FieldNotes...),
	}, nil
}

func LookupDurableContract(name string) (DurableContract, error) {
	schema, err := LookupDurableSurfaceSchema(name)
	if err != nil {
		return DurableContract{}, err
	}
	return DurableContract{
		Surface:            schema.Surface,
		Class:              schema.Class,
		WriteMode:          schema.WriteMode,
		Strict:             schema.Strict,
		FrameworkReadsBody: schema.FrameworkReadsBody,
		Format:             schema.Format,
		Summary:            schema.Summary,
		Example:            prettyDurableSchemaExample(schema.Format, schema.Example),
		Notes:              append([]string(nil), schema.FieldNotes...),
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
	b.WriteString(fmt.Sprintf("- Format: `%s`\n", contract.Format))
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
	if strings.TrimSpace(contract.Example) != "" {
		b.WriteString("\nExample:\n")
		b.WriteString("```")
		b.WriteString(string(contract.Format))
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
		return fmt.Sprintf("goalx durable replace %s --run NAME --file /abs/path.json", contract.Surface)
	case DurableSurfaceClassEventLog:
		return fmt.Sprintf("goalx durable append %s --run NAME --file /abs/path.jsonl", contract.Surface)
	}
	return ""
}

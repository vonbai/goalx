package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

const schemaUsage = "usage: goalx schema <surface> [--json]"

func Schema(projectRoot string, args []string) error {
	if printUsageIfHelp(args, schemaUsage) {
		return nil
	}
	surface, jsonOutput, err := parseSchemaArgs(args)
	if err != nil {
		return err
	}
	if err := rejectLegacyPublicSurface(surface); err != nil {
		return err
	}
	contract, err := LookupDurableContract(surface)
	if err != nil {
		return err
	}
	if jsonOutput {
		data, err := json.MarshalIndent(contract, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	text, err := RenderDurableContract(surface)
	if err != nil {
		return err
	}
	fmt.Print(text)
	return nil
}

func rejectLegacyPublicSurface(surface string) error {
	switch strings.TrimSpace(surface) {
	case "goal":
		return fmt.Errorf("durable surface %q is deprecated; use `obligation-model`", surface)
	case "acceptance":
		return fmt.Errorf("durable surface %q is deprecated; use `assurance-plan`", surface)
	case "goal-log":
		return fmt.Errorf("durable surface %q is deprecated; use `obligation-log`", surface)
	default:
		return nil
	}
}

func parseSchemaArgs(args []string) (surface string, jsonOutput bool, err error) {
	positionals := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--help", "-h", "help":
			return "", false, fmt.Errorf(schemaUsage)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return "", false, fmt.Errorf(schemaUsage)
	}
	return positionals[0], jsonOutput, nil
}

package cli

import (
	"fmt"
	"os"
	"strings"
)

func Durable(projectRoot string, args []string) error {
	if len(args) == 1 && isHelpToken(args[0]) {
		fmt.Println("usage: goalx durable <replace|append> <surface> --run NAME --file /abs/path")
		fmt.Println("inspect the canonical contract first with `goalx schema <surface>`")
		return nil
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: goalx durable <replace|append> <surface> --run NAME --file /abs/path")
	}
	mode := strings.TrimSpace(args[0])
	runName, rest, err := extractRunFlag(args[1:])
	if err != nil {
		return err
	}
	filePath, rest, err := extractStringFlag(rest, "--file")
	if err != nil {
		return err
	}
	if runName == "" || strings.TrimSpace(filePath) == "" || len(rest) != 1 {
		return fmt.Errorf("usage: goalx durable <replace|append> <surface> --run NAME --file /abs/path")
	}
	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	spec, err := LookupDurableSurface(rest[0])
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	switch mode {
	case string(DurableSurfaceWriteModeReplace):
		return durableReplace(rc.RunDir, spec, data)
	case string(DurableSurfaceWriteModeAppend):
		return durableAppend(rc.RunDir, spec, data)
	default:
		return fmt.Errorf("unknown durable mode %q", mode)
	}
}

func durableReplace(runDir string, spec DurableSurfaceSpec, data []byte) error {
	if spec.WriteMode != DurableSurfaceWriteModeReplace {
		return fmt.Errorf("surface %q does not support replace", spec.Name)
	}
	if spec.Class != DurableSurfaceClassStructuredState {
		return fmt.Errorf("surface %q is not a structured state surface", spec.Name)
	}
	if err := validateDurableStructuredPayload(spec.Name, data); err != nil {
		return err
	}
	return writeFileAtomic(spec.Path(runDir), data, 0o644)
}

func durableAppend(runDir string, spec DurableSurfaceSpec, data []byte) error {
	if spec.WriteMode != DurableSurfaceWriteModeAppend {
		return fmt.Errorf("surface %q does not support append", spec.Name)
	}
	if spec.Class != DurableSurfaceClassEventLog {
		return fmt.Errorf("surface %q is not an event log surface", spec.Name)
	}
	return AppendDurableLog(spec.Path(runDir), spec.Name, data)
}

func validateDurableStructuredPayload(surface DurableSurfaceName, data []byte) error {
	switch surface {
	case DurableSurfaceGoal:
		_, err := parseGoalState(data)
		return err
	case DurableSurfaceAcceptance:
		_, err := parseAcceptanceState(data)
		return err
	case DurableSurfaceCoordination:
		_, err := parseCoordinationState(data)
		return err
	case DurableSurfaceStatus:
		_, err := parseRunStatusRecord(data)
		return err
	default:
		return fmt.Errorf("surface %q is not a structured state surface", surface)
	}
}

func extractStringFlag(args []string, name string) (string, []string, error) {
	value := ""
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == name {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("missing value for %s", name)
			}
			value = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	return value, rest, nil
}

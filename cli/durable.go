package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const durableUsage = "usage: goalx durable write <surface> --run NAME --body-file /abs/path.json [--kind KIND] [--actor ACTOR]"

func Durable(projectRoot string, args []string) error {
	if len(args) == 1 && isHelpToken(args[0]) {
		fmt.Println(durableUsage)
		fmt.Println("inspect the authoring contract first with `goalx schema <surface>`")
		return nil
	}
	if len(args) == 0 {
		return fmt.Errorf(durableUsage)
	}
	mode := strings.TrimSpace(args[0])
	if mode != "write" {
		return fmt.Errorf("unknown durable mode %q", mode)
	}
	runName, rest, err := extractRunFlag(args[1:])
	if err != nil {
		return err
	}
	bodyFile, rest, err := extractStringFlag(rest, "--body-file")
	if err != nil {
		return err
	}
	kind, rest, err := extractStringFlag(rest, "--kind")
	if err != nil {
		return err
	}
	actor, rest, err := extractStringFlag(rest, "--actor")
	if err != nil {
		return err
	}
	if runName == "" || strings.TrimSpace(bodyFile) == "" || len(rest) != 1 {
		return fmt.Errorf(durableUsage)
	}
	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	spec, err := LookupDurableSurface(rest[0])
	if err != nil {
		return err
	}
	data, err := os.ReadFile(bodyFile)
	if err != nil {
		return err
	}
	if spec.Class == DurableSurfaceClassEventLog {
		if strings.TrimSpace(kind) == "" {
			return fmt.Errorf("event-log surface %q requires --kind", spec.Name)
		}
		if strings.TrimSpace(actor) == "" {
			return fmt.Errorf("event-log surface %q requires --actor", spec.Name)
		}
	} else {
		if strings.TrimSpace(kind) != "" {
			return fmt.Errorf("structured surface %q does not accept --kind", spec.Name)
		}
		if strings.TrimSpace(actor) != "" {
			return fmt.Errorf("structured surface %q does not accept --actor", spec.Name)
		}
	}
	if spec.Class == DurableSurfaceClassArtifact {
		return fmt.Errorf("surface %q is not machine-consumed", spec.Name)
	}
	return ApplyDurableMutation(rc.RunDir, DurableMutation{
		Surface: spec.Name,
		Kind:    kind,
		Actor:   actor,
		Body:    json.RawMessage(data),
	})
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

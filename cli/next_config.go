package cli

import (
	"fmt"
	"os"
	"strings"

	goalx "github.com/vonbai/goalx"
)

const maxNextConfigParallel = 10

type nextConfigJSON struct {
	Parallel   int      `json:"parallel,omitempty"`
	Dimensions []string `json:"dimensions,omitempty"`
	Objective  string   `json:"objective,omitempty"`
	Context    []string `json:"context,omitempty"`
}

func validateNextConfig(projectRoot string, nc *nextConfigJSON) *nextConfigJSON {
	if nc == nil {
		return nil
	}
	_ = projectRoot

	validated := *nc
	validated.Objective = strings.TrimSpace(validated.Objective)
	validated.Context = normalizeNextConfigContext(validated.Context)
	validated.Dimensions = normalizeNextConfigDimensions(validated.Dimensions)

	if validated.Parallel < 0 {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.parallel=%d (must be >= 0)\n", validated.Parallel)
		validated.Parallel = 0
	} else if validated.Parallel > maxNextConfigParallel {
		fmt.Fprintf(os.Stderr, "warning: capping next_config.parallel=%d to %d\n", validated.Parallel, maxNextConfigParallel)
		validated.Parallel = maxNextConfigParallel
	}
	if len(validated.Dimensions) > maxNextConfigParallel {
		fmt.Fprintf(os.Stderr, "warning: truncating next_config.dimensions to %d entries\n", maxNextConfigParallel)
		validated.Dimensions = validated.Dimensions[:maxNextConfigParallel]
	}
	if len(validated.Dimensions) > 0 {
		if _, err := goalx.ResolveDimensionSpecs(validated.Dimensions); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.dimensions: %v\n", err)
			validated.Dimensions = nil
		}
	}

	return &validated
}

func normalizeNextConfigDimensions(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeNextConfigContext(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		normalized = append(normalized, path)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

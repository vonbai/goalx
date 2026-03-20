package autoresearch

import "fmt"

// BuiltinStrategies are named research approaches for diversity hints.
var BuiltinStrategies = map[string]string{
	"depth":        "Depth-first: Pick the single most impactful area and go as deep as possible. Run experiments, measure things, trace code paths end-to-end. Prefer one thoroughly verified finding over five shallow ones.",
	"breadth":      "Breadth-first: Scan all dimensions quickly to build a complete map, then prioritize the most surprising or impactful areas for deeper analysis. Cover every major component at least once.",
	"adversarial":  "Adversarial: Your job is to find problems. Look for bugs, design flaws, security issues, race conditions, edge cases, and incorrect assumptions. Challenge every claim the code makes. If something looks fine, try harder to break it.",
	"experimental": "Experimental: Quantify everything. Run benchmarks, measure build times, count lines/functions/dependencies, check test coverage, profile memory. Every finding must have a number attached. No opinions without data.",
	"comparative":  "Comparative: Compare this codebase with industry best practices, similar open-source projects, and established patterns. Identify where it deviates and whether those deviations are intentional strengths or accidental weaknesses.",
	"web":          "Web research: Search the internet for related projects, papers, blog posts, and discussions. Find how others have solved similar problems. Bring external knowledge and fresh perspectives that pure code reading cannot provide.",
	"security":     "Security audit: Analyze from an attacker's perspective. Check for injection vulnerabilities, privilege escalation, unsafe input handling, secrets in code, dependency vulnerabilities, and OWASP top 10 issues.",
	"performance":  "Performance analysis: Profile critical paths, identify bottlenecks, analyze algorithmic complexity, check for unnecessary allocations, measure latency, and find scaling limitations. Focus on what matters at production scale.",
}

// DefaultStrategies returns strategy names for a given parallel count.
func DefaultStrategies(n int) []string {
	switch {
	case n >= 4:
		return []string{"depth", "adversarial", "experimental", "comparative"}
	case n == 3:
		return []string{"depth", "adversarial", "experimental"}
	case n == 2:
		return []string{"depth", "adversarial"}
	default:
		return []string{"depth"}
	}
}

// ResolveStrategies converts strategy names to hint strings.
func ResolveStrategies(names []string) ([]string, error) {
	hints := make([]string, 0, len(names))
	for _, name := range names {
		hint, ok := BuiltinStrategies[name]
		if !ok {
			return nil, fmt.Errorf("unknown strategy %q (available: depth, breadth, adversarial, experimental, comparative, web, security, performance)", name)
		}
		hints = append(hints, hint)
	}
	return hints, nil
}

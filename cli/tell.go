package cli

import (
	"fmt"

	goalx "github.com/vonbai/goalx"
)

const tellUsage = `usage: goalx tell [--run NAME] [master|session-N] "message"`

// Tell writes a durable instruction for the master or a session, then best-effort nudges the target pane.
func Tell(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	target, message, err := parseTellArgs(rest)
	if err != nil {
		return err
	}
	if target == "" && message == "" {
		return nil
	}
	resolvedRun, deliveredTarget, err := deliverTell(projectRoot, runName, target, message, sendAgentNudge)
	if err != nil {
		return err
	}
	if deliveredTarget == "master" {
		fmt.Printf("Told master in run '%s'\n", resolvedRun)
		return nil
	}
	fmt.Printf("Told %s in run '%s'\n", deliveredTarget, resolvedRun)
	return nil
}

// AckGuidance marks the current session guidance version as observed by the subagent.
func AckGuidance(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: goalx ack-guidance [--run NAME] <session-name>")
	}
	sessionName := rest[0]

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	idx, err := parseSessionIndex(sessionName)
	if err != nil {
		return err
	}
	ok, err := hasSessionIndex(rc.RunDir, idx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session %q out of range for run %q", sessionName, rc.Name)
	}
	if err := AckSessionGuidance(rc.RunDir, sessionName); err != nil {
		return err
	}
	if guidance, err := LoadSessionGuidanceState(SessionGuidanceStatePath(rc.RunDir, sessionName)); err == nil && guidance != nil {
		_ = UpsertSessionRuntimeState(rc.RunDir, SessionRuntimeState{
			Name:            sessionName,
			GuidanceVersion: guidance.Version,
			GuidancePending: guidance.Pending,
			LastAckVersion:  guidance.LastAckVersion,
		})
	}
	fmt.Printf("Acknowledged guidance for %s in run '%s'\n", sessionName, rc.Name)
	return nil
}

func parseTellArgs(args []string) (string, string, error) {
	switch len(args) {
	case 1:
		if isHelpToken(args[0]) {
			fmt.Println(tellUsage)
			return "", "", nil
		}
		return "master", args[0], nil
	case 2:
		if isHelpToken(args[0]) || isHelpToken(args[1]) {
			fmt.Println(tellUsage)
			return "", "", nil
		}
		return args[0], args[1], nil
	default:
		return "", "", fmt.Errorf(tellUsage)
	}
}

func deliverTell(projectRoot, runName, target, message string, nudge func(target, engine string) error) (string, string, error) {
	if target == "" && message == "" {
		return "", "", nil
	}
	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return "", "", err
	}
	if target == "" || target == "master" {
		msg, err := AppendMasterInboxMessage(rc.RunDir, "tell", "user", message)
		if err != nil {
			return "", "", err
		}
		if nudge != nil {
			dedupeKey := fmt.Sprintf("master-inbox:%d", msg.ID)
			if _, err := DeliverControlNudge(rc.RunDir, dedupeKey, dedupeKey, rc.TmuxSession+":master", rc.Config.Master.Engine, nudge); err != nil {
				return "", "", err
			}
		}
		return rc.Name, "master", nil
	}

	idx, err := parseSessionIndex(target)
	if err != nil {
		return "", "", err
	}
	ok, err := hasSessionIndex(rc.RunDir, idx)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", fmt.Errorf("session %q out of range for run %q", target, rc.Name)
	}
	if err := WriteSessionGuidance(rc.RunDir, target, message); err != nil {
		return "", "", err
	}
	if guidance, err := LoadSessionGuidanceState(SessionGuidanceStatePath(rc.RunDir, target)); err == nil && guidance != nil {
		_ = UpsertSessionRuntimeState(rc.RunDir, SessionRuntimeState{
			Name:            target,
			GuidanceVersion: guidance.Version,
			GuidancePending: guidance.Pending,
			LastAckVersion:  guidance.LastAckVersion,
		})
	}
	windowName, err := resolveWindowName(rc.Name, target)
	if err != nil {
		return "", "", err
	}
	effective := goalx.EffectiveSessionConfig(rc.Config, idx-1)
	if nudge != nil {
		guidance, err := LoadSessionGuidanceState(SessionGuidanceStatePath(rc.RunDir, target))
		if err != nil {
			return "", "", err
		}
		messageID := fmt.Sprintf("guidance:%s:%d", target, guidance.Version)
		if _, err := DeliverControlNudge(rc.RunDir, messageID, messageID, rc.TmuxSession+":"+windowName, effective.Engine, nudge); err != nil {
			return "", "", err
		}
	}
	return rc.Name, target, nil
}

func isHelpToken(arg string) bool {
	switch arg {
	case "--help", "-h", "help":
		return true
	default:
		return false
	}
}

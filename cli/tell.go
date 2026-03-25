package cli

import "fmt"

const tellUsage = `usage: goalx tell [--run NAME] [--urgent] [master|session-N] "message"`

// Tell writes a durable instruction for the master or a session, then best-effort nudges the target pane.
func Tell(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	target, message, urgent, err := parseTellArgs(rest)
	if err != nil {
		return err
	}
	if target == "" && message == "" {
		return nil
	}
	resolvedRun, deliveredTarget, err := deliverTell(projectRoot, runName, target, message, urgent, sendAgentNudge)
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

// AckSession marks the current session inbox as observed by the subagent.
func AckSession(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: goalx ack-session [--run NAME] <session-name>")
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
	if _, err := AckControlInbox(rc.RunDir, sessionName); err != nil {
		return err
	}
	fmt.Printf("Acknowledged session inbox for %s in run '%s'\n", sessionName, rc.Name)
	return nil
}

func parseTellArgs(args []string) (string, string, bool, error) {
	filtered := make([]string, 0, len(args))
	urgent := false
	for _, arg := range args {
		switch {
		case isHelpToken(arg):
			fmt.Println(tellUsage)
			return "", "", false, nil
		case arg == "--urgent":
			urgent = true
		default:
			filtered = append(filtered, arg)
		}
	}

	switch len(filtered) {
	case 1:
		return "master", filtered[0], urgent, nil
	case 2:
		return filtered[0], filtered[1], urgent, nil
	default:
		return "", "", false, fmt.Errorf(tellUsage)
	}
}

func deliverTell(projectRoot, runName, target, message string, urgent bool, nudge func(target, engine string) error) (string, string, error) {
	if target == "" && message == "" {
		return "", "", nil
	}
	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return "", "", err
	}
	if target == "" || target == "master" {
		msg, err := appendControlInboxMessage(rc.RunDir, "master", "tell", "user", message, urgent)
		if err != nil {
			return "", "", err
		}
		if nudge != nil {
			dedupeKey := fmt.Sprintf("master-inbox:%d", msg.ID)
			_, _ = DeliverControlNudge(rc.RunDir, dedupeKey, dedupeKey, rc.TmuxSession+":master", rc.Config.Master.Engine, nudge)
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
	msg, err := appendControlInboxMessage(rc.RunDir, target, "tell", "user", message, urgent)
	if err != nil {
		return "", "", err
	}
	windowName, err := resolveWindowName(rc.Name, target)
	if err != nil {
		return "", "", err
	}
	identity, err := RequireSessionIdentity(rc.RunDir, target)
	if err != nil {
		return "", "", fmt.Errorf("load %s identity: %w", target, err)
	}
	if nudge != nil {
		messageID := fmt.Sprintf("session-inbox:%s:%d", target, msg.ID)
		_, _ = DeliverControlNudge(rc.RunDir, messageID, messageID, rc.TmuxSession+":"+windowName, identity.Engine, nudge)
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

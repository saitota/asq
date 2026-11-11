package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/saitota/asq/internal/session"
	"github.com/saitota/asq/internal/source"
)

func (a App) resume(args []string) error {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	cachePath := fs.String("cache", "", "message cache path")
	dryRun := fs.Bool("dry-run", false, "print command without exec")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	sessions, err := a.loadSessions(*cachePath, false)
	if err != nil {
		return err
	}
	var sess session.Session
	if fs.NArg() > 0 {
		sess, err = findSession(sessions, selectionKeyFromArgs(fs.Args()))
	} else {
		sess, err = a.pickSession(sessions, "")
	}
	if err != nil {
		return err
	}
	cmd, cmdArgs, err := resumeTarget(a.sources, sess)
	if err != nil {
		return err
	}
	if *dryRun {
		_, _ = fmt.Fprint(a.Stdout, resumePreview(sess.CWD, cmd, cmdArgs))
		return nil
	}
	return a.execResumeCommand(sess.Source, sess.CWD, cmd, cmdArgs)
}

func (a App) pick(args []string) error {
	fs := flag.NewFlagSet("pick", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	cachePath := fs.String("cache", "", "message cache path")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	sessions, err := a.loadSessions(*cachePath, false)
	if err != nil {
		return err
	}
	sess, err := a.pickSession(sessions, strings.Join(fs.Args(), " "))
	if err != nil {
		return err
	}
	cmd, cmdArgs, err := resumeTarget(a.sources, sess)
	if err != nil {
		return err
	}
	return a.execResumeCommand(sess.Source, sess.CWD, cmd, cmdArgs)
}

func resumeTarget(reg source.Registry, sess session.Session) (cmd string, args []string, err error) {
	cmd, args, err = reg.Resume(sess.Source, sess.ID)
	if err != nil {
		return "", nil, err
	}
	if err := validateSessionCWD(sess); err != nil {
		return "", nil, err
	}
	return cmd, args, nil
}

func (a App) execResumeCommand(src session.Source, cwd, cmd string, cmdArgs []string) error {
	path, err := exec.LookPath(cmd)
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%s not found; install it to resume %s sessions", cmd, src)
	}
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(a.Stderr, "asq: %s", resumePreview(cwd, cmd, cmdArgs))
	resume := exec.Command(path, cmdArgs...)
	resume.Dir = cwd
	resume.Stdin = a.Stdin
	resume.Stdout = a.Stdout
	resume.Stderr = a.Stderr
	return resume.Run()
}

func validateSessionCWD(sess session.Session) error {
	if !sess.HasCWD() {
		return fmt.Errorf("refusing to use session without a decoded cwd: %s", sess.ID)
	}
	info, err := os.Stat(sess.CWD)
	if err != nil {
		return fmt.Errorf("cannot access session cwd %q: %w", sess.CWD, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("session cwd is not a directory: %s", sess.CWD)
	}
	return nil
}

func resumePreview(cwd, cmd string, args []string) string {
	argv := append([]string{cmd}, args...)
	cwdJSON, _ := json.Marshal(cwd)
	argvJSON, _ := json.Marshal(argv)
	return fmt.Sprintf("cwd: %s\nargv: %s\n", cwdJSON, argvJSON)
}

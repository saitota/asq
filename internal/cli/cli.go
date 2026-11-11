package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/saitota/asq/internal/agentsview"
	"github.com/saitota/asq/internal/buildinfo"
	"github.com/saitota/asq/internal/progress"
	"github.com/saitota/asq/internal/session"
	"github.com/saitota/asq/internal/source"
	"github.com/saitota/asq/internal/tui"
)

const defaultListLimit = 50

type App struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	// Pick chooses one item interactively; tests may replace it.
	Pick func(items []tui.Item, preview func(index int) string, query string) (int, error)

	sources source.Registry
}

func New() App {
	return App{Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin, Pick: tui.Pick}
}

func (a App) Run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "version", "-v", "--version":
			return a.version()
		case "help", "-h", "--help":
			return a.usage()
		}
	}
	a.sources = a.loadSources()
	if len(args) == 0 {
		return a.pick(nil)
	}
	switch args[0] {
	case "reindex":
		return a.reindex(args[1:])
	case "list", "ls":
		return a.list(args[1:])
	case "resume":
		return a.resume(args[1:])
	case "pick":
		return a.pick(args[1:])
	case "cwd":
		return a.cwd(args[1:])
	case "preview":
		return a.preview(args[1:])
	default:
		return a.pick(args)
	}
}

func (a App) loadSources() source.Registry {
	reg, err := source.Load()
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "asq: ignoring user-defined agents: %v\n", err)
		return source.Builtin()
	}
	return reg
}

func (a App) usage() error {
	_, _ = fmt.Fprintln(a.Stdout, "usage: asq [query]           # pick a session and resume it from its directory")
	_, _ = fmt.Fprintln(a.Stdout, "       asq pick [query]")
	_, _ = fmt.Fprintln(a.Stdout, "       asq <reindex|list|pick|cwd|resume|version> [options]")
	return nil
}

func (a App) version() error {
	_, err := fmt.Fprintln(a.Stdout, buildinfo.DisplayVersion())
	return err
}

func (a App) reindex(args []string) error {
	fs := flag.NewFlagSet("reindex", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	cachePath := fs.String("cache", "", "message cache path")
	force := fs.Bool("force", false, "refetch every conversation, ignoring the cache")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	sessions, err := a.loadSessions(*cachePath, *force)
	if err != nil {
		return err
	}
	failed := countSessionsWithErrors(sessions)
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(map[string]int{"sessions": len(sessions), "errors": failed})
	}
	line := fmt.Sprintf("sessions=%d", len(sessions))
	if failed > 0 {
		line += fmt.Sprintf(" errors=%d", failed)
	}
	_, _ = fmt.Fprintln(a.Stdout, line)
	return nil
}

func (a App) list(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	sourceFlag := fs.String("source", "", "source filter, e.g. "+a.sources.List())
	cachePath := fs.String("cache", "", "message cache path")
	cwd := fs.String("cwd", "", "cwd substring filter")
	allDirs := fs.Bool("all", false, "show sessions from all directories")
	limit := fs.Int("limit", defaultListLimit, "max rows")
	jsonOut := fs.Bool("json", false, "output JSON")
	fzfOut := fs.Bool("fzf", false, "output fzf-friendly TSV")
	verbose := fs.Bool("verbose", false, "output verbose rows")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	sourceFilter := session.Source(*sourceFlag)
	limitValue := *limit
	limitSet := false
	cwdSet := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "limit":
			limitSet = true
		case "cwd":
			cwdSet = true
		}
	})
	if !limitSet && (*jsonOut || *fzfOut) {
		limitValue = 0
	}
	if *allDirs && cwdSet {
		return errors.New("cannot use --all with --cwd")
	}
	cwdFilter := *cwd
	cwdExact := false
	if !*allDirs && !cwdSet {
		current, err := os.Getwd()
		if err != nil {
			return err
		}
		cwdFilter = current
		cwdExact = true
	}
	sessions, err := a.loadSessions(*cachePath, false)
	if err != nil {
		return err
	}
	if failed := countSessionsWithErrors(sessions); failed > 0 {
		_, _ = fmt.Fprintf(a.Stderr, "asq: %d sessions have incomplete conversations; run asq reindex to retry\n", failed)
	}
	sessions = session.Filter(sessions, session.FilterOptions{Source: sourceFilter, CWD: cwdFilter, CWDExact: cwdExact, Limit: limitValue})
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(newListOutput(sessions))
	}
	for _, sess := range sessions {
		if *fzfOut {
			_, _ = fmt.Fprintln(a.Stdout, newSelectionLine(a.sources, sess, true).encode())
			continue
		}
		if *verbose {
			_, _ = fmt.Fprintf(a.Stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", sess.Source, sessionTime(sess), sess.Name, sess.ID, sess.CWD, sess.GitBranch)
			continue
		}
	}
	if *fzfOut || *verbose {
		return nil
	}
	return writeSessionTable(a.Stdout, a.sources, sessions)
}

func (a App) cwd(args []string) error {
	fs := flag.NewFlagSet("cwd", flag.ContinueOnError)
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
	var sess session.Session
	if fs.NArg() > 0 {
		sess, err = findSession(sessions, selectionKeyFromArgs(fs.Args()))
	} else {
		sess, err = a.pickSession(sessions, "")
	}
	if err != nil {
		return err
	}
	if err := validateSessionCWD(sess); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(a.Stdout, sess.CWD)
	return nil
}

func (a App) preview(args []string) error {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	cachePath := fs.String("cache", "", "message cache path")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("preview requires a session id")
	}
	sessions, err := a.loadSessions(*cachePath, false)
	if err != nil {
		return err
	}
	sess, err := findSession(sessions, selectionKeyFromArgs(fs.Args()))
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(a.Stdout, previewText(sess))
	return nil
}

func (a App) loadSessions(cachePath string, force bool) ([]session.Session, error) {
	if cachePath == "" {
		var err error
		cachePath, err = agentsview.DefaultCachePath()
		if err != nil {
			return nil, err
		}
	}
	return agentsview.Client{CachePath: cachePath, IgnoreCache: force, Reporter: progress.NewReporter(a.Stderr)}.Sessions()
}

func countSessionsWithErrors(sessions []session.Session) int {
	count := 0
	for _, sess := range sessions {
		if len(sess.Errors) > 0 {
			count++
		}
	}
	return count
}

// findSession matches ids opaquely; resume validates before exec.
func findSession(sessions []session.Session, id string) (session.Session, error) {
	if id == "" {
		return session.Session{}, errors.New("empty session id")
	}
	sess, ok := session.FindByID(sessions, id)
	if !ok {
		return session.Session{}, fmt.Errorf("session not found or ambiguous: %s", id)
	}
	return sess, nil
}

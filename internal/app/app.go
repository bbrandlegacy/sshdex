package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bbrandlegacy/sshdex/internal/profile"
	"github.com/bbrandlegacy/sshdex/internal/security"
	"github.com/bbrandlegacy/sshdex/internal/sshcmd"
	"github.com/bbrandlegacy/sshdex/internal/sshconfig"
	"github.com/bbrandlegacy/sshdex/internal/store"
)

const Version = "0.5.0"

type options struct {
	storePath string
	args      []string
}

func Run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseGlobal(args)
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if opts.storePath == "" {
		opts.storePath = defaultStorePath()
	}
	if len(opts.args) == 0 {
		printHelp(stdout)
		return 0
	}
	cmd := opts.args[0]
	rest := opts.args[1:]
	switch cmd {
	case "help", "--help", "-h":
		printHelp(stdout)
		return 0
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, "sshdex "+Version)
		return 0
	case "add":
		return runAdd(opts.storePath, rest, stdout, stderr)
	case "list":
		return runList(opts.storePath, rest, stdout, stderr)
	case "show":
		return runShow(opts.storePath, rest, stdout, stderr)
	case "edit":
		return runEdit(opts.storePath, rest, stdout, stderr)
	case "delete":
		return runDelete(opts.storePath, rest, stdout, stderr)
	case "connect":
		return runConnect(opts.storePath, rest, stdout, stderr)
	case "pick":
		return runPick(opts.storePath, rest, stdout, stderr)
	case "import":
		return runImport(opts.storePath, rest, stdout, stderr)
	case "export":
		return runExport(opts.storePath, rest, stdout, stderr)
	case "backup":
		return runBackup(opts.storePath, rest, stdout, stderr)
	case "completion":
		return runCompletion(rest, stdout, stderr)
	case "doctor":
		return runDoctor(opts.storePath, stdout, stderr)
	default:
		// Shorthand connect: sshdex <name> [--dry-run]
		return runConnect(opts.storePath, append([]string{cmd}, rest...), stdout, stderr)
	}
}

func parseGlobal(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		if args[i] == "--store" {
			if i+1 >= len(args) {
				return opts, errors.New("--store requires a path")
			}
			opts.storePath = args[i+1]
			i++
			continue
		}
		opts.args = append(opts.args, args[i])
	}
	return opts, nil
}

func defaultStorePath() string {
	if v := os.Getenv("SSHDex_STORE"); v != "" {
		return v
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return filepath.Join(".", "profiles.json")
	}
	return filepath.Join(base, "sshdex", "profiles.json")
}

func printError(w io.Writer, err error) {
	fmt.Fprintln(w, security.ErrorString(err))
}

func safeText(text string) string {
	return security.Redact(text)
}

func safeJoin(values []string, sep string) string {
	redacted := make([]string, 0, len(values))
	for _, value := range values {
		redacted = append(redacted, safeText(value))
	}
	return strings.Join(redacted, sep)
}

func runAdd(path string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	p, tags, locals, remotes, dynamics := bindProfileFlags(fs, profile.Profile{})
	if len(args) == 0 {
		interactive, err := promptProfile(stdout, os.Stdin, profile.Profile{}, true)
		if err != nil {
			printError(stderr, err)
			return 1
		}
		p = &interactive
	} else {
		if err := fs.Parse(args); err != nil {
			return 2
		}
		p.Tags = tags.Values
		p.LocalForwards = locals.Values
		p.RemoteForwards = remotes.Values
		p.DynamicForwards = dynamics.Values
	}
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	if err := s.Add(*p); err != nil {
		printError(stderr, err)
		return 1
	}
	if err := s.Save(); err != nil {
		printError(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "added %s\n", strings.TrimSpace(p.Name))
	return 0
}

func runList(path string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tag := fs.String("tag", "", "filter by tag")
	search := fs.String("search", "", "search name/host/tag")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	for _, p := range s.List() {
		if *tag != "" && !hasTag(p, *tag) {
			continue
		}
		if *search != "" && !matchesSearch(p, *search) {
			continue
		}
		fmt.Fprintf(stdout, "%s	%s	%s	%s\n", safeText(p.Name), safeText(p.Host), safeText(p.User), safeJoin(p.Tags, ","))
	}
	return 0
}

func runShow(path string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "show requires NAME")
		return 2
	}
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	p, err := s.Get(args[0])
	if err != nil {
		printError(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Name: %s\nHost: %s\nUser: %s\nPort: %d\nIdentityFile: %s\nProxyJump: %s\nLocalForwards: %s\nRemoteForwards: %s\nDynamicForwards: %s\nRemoteCommand: %s\nTags: %s\nNotes: %s\nConnectionCount: %d\n", safeText(p.Name), safeText(p.Host), safeText(p.User), p.Port, safeText(p.IdentityFile), safeText(p.ProxyJump), safeJoin(p.LocalForwards, ","), safeJoin(p.RemoteForwards, ","), safeJoin(p.DynamicForwards, ","), safeText(p.RemoteCommand), safeJoin(p.Tags, ","), safeText(p.Notes), p.ConnectionCount)
	return 0
}

func runEdit(path string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "edit requires NAME")
		return 2
	}
	name := args[0]
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	existing, err := s.Get(name)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	p, tags, locals, remotes, dynamics := bindProfileFlags(fs, existing)
	if len(args) == 1 {
		interactive, err := promptProfile(stdout, os.Stdin, existing, false)
		if err != nil {
			printError(stderr, err)
			return 1
		}
		p = &interactive
	} else {
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if tags.Seen {
			p.Tags = tags.Values
		}
		if locals.Seen {
			p.LocalForwards = locals.Values
		}
		if remotes.Seen {
			p.RemoteForwards = remotes.Values
		}
		if dynamics.Seen {
			p.DynamicForwards = dynamics.Values
		}
	}
	p.Name = existing.Name
	p.CreatedAt = existing.CreatedAt
	p.UpdatedAt = time.Now().UTC()
	p.LastConnectedAt = existing.LastConnectedAt
	p.ConnectionCount = existing.ConnectionCount
	if err := s.Update(*p); err != nil {
		printError(stderr, err)
		return 1
	}
	if err := s.Save(); err != nil {
		printError(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "updated %s\n", p.Name)
	return 0
}

func runDelete(path string, args []string, stdout, stderr io.Writer) int {
	force := false
	names := []string{}
	for _, arg := range args {
		switch arg {
		case "--force":
			force = true
		default:
			names = append(names, arg)
		}
	}
	if len(names) != 1 {
		fmt.Fprintln(stderr, "delete requires NAME")
		return 2
	}
	if !force {
		if !confirm(stdout, os.Stdin, fmt.Sprintf("Delete %s?", safeText(names[0]))) {
			fmt.Fprintln(stdout, "cancelled")
			return 0
		}
	}
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if err := s.Delete(names[0]); err != nil {
		printError(stderr, err)
		return 1
	}
	if err := s.Save(); err != nil {
		printError(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "deleted %s\n", safeText(names[0]))
	return 0
}

func runConnect(path string, args []string, stdout, stderr io.Writer) int {
	dryRun := false
	names := []string{}
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		default:
			names = append(names, arg)
		}
	}
	if len(names) != 1 {
		fmt.Fprintln(stderr, "connect requires NAME")
		return 2
	}
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	p, err := s.Get(names[0])
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if dryRun {
		preview, err := sshcmd.Preview(p)
		if err != nil {
			printError(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, preview)
		return 0
	}
	argv, err := sshcmd.BuildArgs(p)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	now := time.Now().UTC()
	p.LastConnectedAt = &now
	p.ConnectionCount++
	_ = s.Update(p)
	_ = s.Save()
	c := exec.Command("ssh", argv...)
	c.Stdout = stdout
	c.Stderr = stderr
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		printError(stderr, err)
		return 1
	}
	return 0
}

func runPick(path string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pick", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tag := fs.String("tag", "", "filter by tag")
	search := fs.String("search", "", "search name/host/tag")
	index := fs.Int("index", 0, "1-based selected profile index")
	dryRun := fs.Bool("dry-run", false, "print safe SSH preview instead of launching ssh")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	matches := []profile.Profile{}
	for _, p := range s.List() {
		if *tag != "" && !hasTag(p, *tag) {
			continue
		}
		if *search != "" && !matchesSearch(p, *search) {
			continue
		}
		matches = append(matches, p)
	}
	if *index == 0 {
		for i, p := range matches {
			fmt.Fprintf(stdout, "%d	%s	%s	%s	%s\n", i+1, safeText(p.Name), safeText(p.Host), safeText(p.User), safeJoin(p.Tags, ","))
		}
		return 0
	}
	if *index < 1 || *index > len(matches) {
		fmt.Fprintf(stderr, "pick index %d out of range for %d match(es)\n", *index, len(matches))
		return 2
	}
	selected := matches[*index-1]
	connectArgs := []string{selected.Name}
	if *dryRun {
		connectArgs = append(connectArgs, "--dry-run")
	}
	return runConnect(path, connectArgs, stdout, stderr)
}

func runDoctor(path string, stdout, stderr io.Writer) int {
	diagnostics := store.SecurityDiagnostics(path)
	s, err := store.Load(path)
	if err != nil {
		for _, diagnostic := range diagnostics {
			fmt.Fprintf(stdout, "StoreSecurity: %s: %s (fix: %s)\n", diagnostic.Severity, safeText(diagnostic.Message), safeText(diagnostic.Fix))
		}
		printError(stderr, err)
		return 1
	}
	sshPath, err := exec.LookPath("ssh")
	sshStatus := "missing"
	if err == nil {
		sshStatus = sshPath
	}
	invalid := doctorInvalidProfiles(s.List())
	fmt.Fprintf(stdout, "Store: %s\nProfiles: %d\nSSH: %s\nInvalidProfiles: %d\nStoreSecurityFindings: %d\n", safeText(path), len(s.List()), safeText(sshStatus), len(invalid), len(diagnostics))
	for _, diagnostic := range diagnostics {
		fmt.Fprintf(stdout, "- %s: %s (fix: %s)\n", diagnostic.Severity, safeText(diagnostic.Message), safeText(diagnostic.Fix))
	}
	for _, msg := range invalid {
		if len(diagnostics) > 0 {
			break
		}
		fmt.Fprintf(stdout, "- %s\n", safeText(msg))
	}
	return 0
}

func doctorInvalidProfiles(profiles []profile.Profile) []string {
	var invalid []string
	for _, p := range profiles {
		if p.IdentityFile != "" {
			expanded := expandHome(p.IdentityFile)
			if _, err := os.Stat(expanded); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					invalid = append(invalid, fmt.Sprintf("%s: missing identity file %s", p.Name, p.IdentityFile))
				}
			}
		}
	}
	return invalid
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func runImport(path string, args []string, stdout, stderr io.Writer) int {
	opts, err := parseImportArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, safeText(err.Error()))
		return 2
	}
	if opts.path == "" {
		label := "SSH config path"
		if opts.format == importFormatSSHDex {
			label = "sshdex JSON path"
		}
		inputPath, err := promptLine(stdout, bufio.NewReader(os.Stdin), label, "")
		if err != nil {
			printError(stderr, err)
			return 1
		}
		if strings.TrimSpace(inputPath) != "" {
			opts.path = inputPath
		}
	}
	if opts.path == "" {
		fmt.Fprintln(stderr, "import requires PATH")
		return 2
	}
	data, err := os.ReadFile(opts.path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	profiles, err := parseImportProfiles(data, opts.format)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	plan, summary, err := planProfileImport(s, profiles, opts.conflict)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if opts.dryRun {
		for _, item := range plan {
			fmt.Fprintf(stdout, "%s	%s	%s	%s\n", safeText(item.Profile.Name), safeText(item.Profile.Host), safeText(item.Profile.User), safeText(item.Description()))
		}
		fmt.Fprintf(stdout, "dry-run: would import %d profile(s), replace %d, rename %d, skip %d duplicate(s)\n", summary.Imported, summary.Replaced, summary.Renamed, summary.Skipped)
		return 0
	}
	if err := applyProfileImport(s, plan); err != nil {
		printError(stderr, err)
		return 1
	}
	if summary.Imported > 0 || summary.Replaced > 0 {
		if err := s.Save(); err != nil {
			printError(stderr, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "imported %d profile(s), replaced %d, renamed %d, skipped %d duplicate(s)\n", summary.Imported, summary.Replaced, summary.Renamed, summary.Skipped)
	return 0
}

type importFormat string

const (
	importFormatOpenSSH importFormat = "openssh"
	importFormatSSHDex  importFormat = "sshdex"
)

type conflictPolicy string

const (
	conflictSkip    conflictPolicy = "skip"
	conflictReplace conflictPolicy = "replace"
	conflictRename  conflictPolicy = "rename"
)

type importOptions struct {
	dryRun   bool
	format   importFormat
	conflict conflictPolicy
	path     string
}

func parseImportArgs(args []string) (importOptions, error) {
	opts := importOptions{format: importFormatOpenSSH, conflict: conflictSkip}
	paths := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			opts.dryRun = true
		case arg == "--format":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--format requires openssh or sshdex")
			}
			opts.format = importFormat(args[i+1])
			i++
		case strings.HasPrefix(arg, "--format="):
			opts.format = importFormat(strings.TrimPrefix(arg, "--format="))
		case arg == "--conflict":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--conflict requires skip, replace, or rename")
			}
			opts.conflict = conflictPolicy(args[i+1])
			i++
		case strings.HasPrefix(arg, "--conflict="):
			opts.conflict = conflictPolicy(strings.TrimPrefix(arg, "--conflict="))
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown import flag %s", arg)
		default:
			paths = append(paths, arg)
		}
	}
	if opts.format != importFormatOpenSSH && opts.format != importFormatSSHDex {
		return opts, fmt.Errorf("unsupported import format %s", opts.format)
	}
	if opts.conflict != conflictSkip && opts.conflict != conflictReplace && opts.conflict != conflictRename {
		return opts, fmt.Errorf("unsupported conflict policy %s", opts.conflict)
	}
	if len(paths) > 1 {
		return opts, fmt.Errorf("import requires PATH")
	}
	if len(paths) == 1 {
		opts.path = paths[0]
	}
	return opts, nil
}

func parseImportProfiles(data []byte, format importFormat) ([]profile.Profile, error) {
	switch format {
	case importFormatOpenSSH:
		return sshconfig.ParseString(string(data))
	case importFormatSSHDex:
		profiles, err := readProfilesData(data)
		if err != nil {
			return nil, err
		}
		out := make([]profile.Profile, 0, len(profiles))
		for _, p := range profiles {
			normalized, err := profile.Validate(p)
			if err != nil {
				return nil, err
			}
			out = append(out, normalized)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported import format %s", format)
	}
}

func readProfilesData(data []byte) ([]profile.Profile, error) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "[") {
		var profiles []profile.Profile
		if err := json.Unmarshal(data, &profiles); err != nil {
			return nil, err
		}
		return profiles, nil
	}
	var exported exportedProfiles
	if err := json.Unmarshal(data, &exported); err != nil {
		return nil, err
	}
	return exported.Profiles, nil
}

type importAction string

const (
	importActionImport  importAction = "import"
	importActionSkip    importAction = "skip"
	importActionReplace importAction = "replace"
	importActionRename  importAction = "rename"
)

type profileImportPlanItem struct {
	Action       importAction
	OriginalName string
	Profile      profile.Profile
}

func (item profileImportPlanItem) Description() string {
	if item.Action == importActionRename {
		return fmt.Sprintf("rename:%s", item.OriginalName)
	}
	return string(item.Action)
}

type profileImportSummary struct {
	Imported int
	Replaced int
	Renamed  int
	Skipped  int
}

func planProfileImport(s *store.Store, profiles []profile.Profile, policy conflictPolicy) ([]profileImportPlanItem, profileImportSummary, error) {
	used := map[string]struct{}{}
	for _, p := range s.List() {
		used[p.Name] = struct{}{}
	}
	plan := make([]profileImportPlanItem, 0, len(profiles))
	var summary profileImportSummary
	for _, candidate := range profiles {
		p, err := profile.Validate(candidate)
		if err != nil {
			return nil, summary, err
		}
		originalName := p.Name
		_, exists := used[p.Name]
		switch {
		case !exists:
			used[p.Name] = struct{}{}
			plan = append(plan, profileImportPlanItem{Action: importActionImport, OriginalName: originalName, Profile: p})
			summary.Imported++
		case policy == conflictSkip:
			plan = append(plan, profileImportPlanItem{Action: importActionSkip, OriginalName: originalName, Profile: p})
			summary.Skipped++
		case policy == conflictReplace:
			used[p.Name] = struct{}{}
			plan = append(plan, profileImportPlanItem{Action: importActionReplace, OriginalName: originalName, Profile: p})
			summary.Replaced++
		case policy == conflictRename:
			p.Name = uniqueImportedName(originalName, used)
			p, err = profile.Validate(p)
			if err != nil {
				return nil, summary, err
			}
			used[p.Name] = struct{}{}
			plan = append(plan, profileImportPlanItem{Action: importActionRename, OriginalName: originalName, Profile: p})
			summary.Imported++
			summary.Renamed++
		}
	}
	return plan, summary, nil
}

func uniqueImportedName(base string, used map[string]struct{}) string {
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func applyProfileImport(s *store.Store, plan []profileImportPlanItem) error {
	for _, item := range plan {
		switch item.Action {
		case importActionSkip:
			continue
		case importActionImport, importActionRename:
			if err := s.Add(item.Profile); err != nil {
				return err
			}
		case importActionReplace:
			if _, err := s.Get(item.Profile.Name); err != nil {
				if errors.Is(err, store.ErrProfileNotFound) {
					if addErr := s.Add(item.Profile); addErr != nil {
						return addErr
					}
					continue
				}
				return err
			}
			if err := s.Update(item.Profile); err != nil {
				return err
			}
		}
	}
	return nil
}

func promptProfile(stdout io.Writer, stdin *os.File, base profile.Profile, requireName bool) (profile.Profile, error) {
	reader := bufio.NewReader(stdin)
	p := base
	var err error
	if requireName {
		p.Name, err = promptLine(stdout, reader, "Name", p.Name)
		if err != nil {
			return profile.Profile{}, err
		}
	}
	if p.Host, err = promptLine(stdout, reader, "Host", p.Host); err != nil {
		return profile.Profile{}, err
	}
	if p.User, err = promptLine(stdout, reader, "User", p.User); err != nil {
		return profile.Profile{}, err
	}
	portDefault := ""
	if p.Port != 0 {
		portDefault = strconv.Itoa(p.Port)
	}
	portText, err := promptLine(stdout, reader, "Port", portDefault)
	if err != nil {
		return profile.Profile{}, err
	}
	if strings.TrimSpace(portText) != "" {
		p.Port, err = strconv.Atoi(strings.TrimSpace(portText))
		if err != nil {
			return profile.Profile{}, fmt.Errorf("invalid port %q", portText)
		}
	}
	if p.IdentityFile, err = promptLine(stdout, reader, "Identity file", p.IdentityFile); err != nil {
		return profile.Profile{}, err
	}
	tagText, err := promptLine(stdout, reader, "Tags (comma-separated)", strings.Join(p.Tags, ","))
	if err != nil {
		return profile.Profile{}, err
	}
	p.Tags = splitCSV(tagText)
	if p.Notes, err = promptLine(stdout, reader, "Notes", p.Notes); err != nil {
		return profile.Profile{}, err
	}
	if p.ProxyJump, err = promptLine(stdout, reader, "ProxyJump", p.ProxyJump); err != nil {
		return profile.Profile{}, err
	}
	localText, err := promptLine(stdout, reader, "Local forwards (-L, comma-separated)", strings.Join(p.LocalForwards, ","))
	if err != nil {
		return profile.Profile{}, err
	}
	p.LocalForwards = splitCSV(localText)
	remoteText, err := promptLine(stdout, reader, "Remote forwards (-R, comma-separated)", strings.Join(p.RemoteForwards, ","))
	if err != nil {
		return profile.Profile{}, err
	}
	p.RemoteForwards = splitCSV(remoteText)
	dynamicText, err := promptLine(stdout, reader, "Dynamic forwards (-D, comma-separated)", strings.Join(p.DynamicForwards, ","))
	if err != nil {
		return profile.Profile{}, err
	}
	p.DynamicForwards = splitCSV(dynamicText)
	if p.RemoteCommand, err = promptLine(stdout, reader, "Remote command", p.RemoteCommand); err != nil {
		return profile.Profile{}, err
	}
	return p, nil
}

func promptLine(stdout io.Writer, reader *bufio.Reader, label, defaultValue string) (string, error) {
	if defaultValue == "" {
		fmt.Fprintf(stdout, "%s: ", label)
	} else {
		fmt.Fprintf(stdout, "%s [%s]: ", label, defaultValue)
	}
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func confirm(stdout io.Writer, stdin *os.File, question string) bool {
	reader := bufio.NewReader(stdin)
	fmt.Fprintf(stdout, "%s [y/N]: ", question)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func runCompletion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "completion requires SHELL: bash, zsh, or fish")
		return 2
	}
	switch args[0] {
	case "bash":
		fmt.Fprint(stdout, bashCompletion())
	case "zsh":
		fmt.Fprint(stdout, zshCompletion())
	case "fish":
		fmt.Fprint(stdout, fishCompletion())
	default:
		fmt.Fprintln(stderr, "unsupported shell: "+safeText(args[0]))
		return 2
	}
	return 0
}

func completionCommands() string {
	return "add list show edit delete connect pick import export backup completion doctor version help"
}

func bashCompletion() string {
	return fmt.Sprintf(`# sshdex bash completion
_sshdex_completion() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  if [[ COMP_CWORD -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "%s" -- "$cur") )
  fi
}
complete -F _sshdex_completion sshdex
`, completionCommands())
}

func zshCompletion() string {
	return fmt.Sprintf(`#compdef sshdex
# sshdex zsh completion
_arguments '1:command:(%s)'
`, completionCommands())
}

func fishCompletion() string {
	var b strings.Builder
	b.WriteString("# sshdex fish completion\n")
	for _, cmd := range strings.Fields(completionCommands()) {
		fmt.Fprintf(&b, "complete -c sshdex -f -n '__fish_use_subcommand' -a %s\n", cmd)
	}
	return b.String()
}

func runExport(path string, args []string, stdout, stderr io.Writer) int {
	dest, err := onePathOrPrompt(args, stdout, os.Stdin, "Export path", "")
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if strings.TrimSpace(dest) == "" {
		fmt.Fprintln(stderr, "export requires PATH")
		return 2
	}
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if err := writeProfilesFile(dest, s.List()); err != nil {
		printError(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "exported %d profile(s) to %s\n", len(s.List()), safeText(dest))
	return 0
}

func runBackup(path string, args []string, stdout, stderr io.Writer) int {
	defaultPath := path + ".bak"
	dest, err := onePathOrPrompt(args, stdout, os.Stdin, "Backup path", defaultPath)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if strings.TrimSpace(dest) == "" {
		dest = defaultPath
	}
	s, err := store.Load(path)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if err := writeProfilesFile(dest, s.List()); err != nil {
		printError(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "backup written to %s\n", safeText(dest))
	return 0
}

func onePathOrPrompt(args []string, stdout io.Writer, stdin *os.File, label, defaultValue string) (string, error) {
	paths := []string{}
	for _, arg := range args {
		paths = append(paths, arg)
	}
	if len(paths) == 0 {
		return promptLine(stdout, bufio.NewReader(stdin), label, defaultValue)
	}
	if len(paths) != 1 {
		return "", fmt.Errorf("expected one path")
	}
	return paths[0], nil
}

type exportedProfiles struct {
	Profiles []profile.Profile `json:"profiles"`
}

func writeProfilesFile(path string, profiles []profile.Profile) error {
	data, err := json.MarshalIndent(exportedProfiles{Profiles: profiles}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".sshdex-export-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

type tagValues struct {
	Values []string
	Seen   bool
}

func (t *tagValues) String() string     { return strings.Join(t.Values, ",") }
func (t *tagValues) Set(v string) error { t.Seen = true; t.Values = append(t.Values, v); return nil }

func bindProfileFlags(fs *flag.FlagSet, base profile.Profile) (*profile.Profile, *tagValues, *tagValues, *tagValues, *tagValues) {
	p := base
	tags := &tagValues{}
	localForwards := &tagValues{}
	remoteForwards := &tagValues{}
	dynamicForwards := &tagValues{}
	fs.StringVar(&p.Name, "name", base.Name, "profile name")
	fs.StringVar(&p.Host, "host", base.Host, "host")
	fs.StringVar(&p.User, "user", base.User, "user")
	fs.IntVar(&p.Port, "port", base.Port, "port")
	fs.StringVar(&p.IdentityFile, "identity-file", base.IdentityFile, "identity file")
	fs.StringVar(&p.ProxyJump, "proxy-jump", base.ProxyJump, "proxy jump")
	fs.StringVar(&p.RemoteCommand, "remote-command", base.RemoteCommand, "remote command to run after connecting")
	fs.StringVar(&p.Notes, "notes", base.Notes, "notes")
	fs.Var(tags, "tag", "tag; may repeat")
	fs.Var(localForwards, "local-forward", "local SSH forward (-L); may repeat")
	fs.Var(remoteForwards, "remote-forward", "remote SSH forward (-R); may repeat")
	fs.Var(dynamicForwards, "dynamic-forward", "dynamic SOCKS SSH forward (-D); may repeat")
	return &p, tags, localForwards, remoteForwards, dynamicForwards
}

func hasTag(p profile.Profile, tag string) bool {
	for _, t := range p.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}
func matchesSearch(p profile.Profile, q string) bool {
	q = strings.ToLower(q)
	fields := []string{p.Name, p.Host, p.User, p.Notes, p.ProxyJump, p.RemoteCommand, strconv.Itoa(p.Port)}
	fields = append(fields, p.Tags...)
	fields = append(fields, p.LocalForwards...)
	fields = append(fields, p.RemoteForwards...)
	fields = append(fields, p.DynamicForwards...)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `sshdex - local-first SSH profile manager

Commands:
  add      Add a profile
  list     List profiles
  show     Show a profile
  edit     Edit a profile
  delete   Delete a profile
  connect  Connect to a profile
  pick     Search/select a profile by number
  import   Import profiles from OpenSSH config or sshdex JSON
  export   Export profiles to a JSON file
  backup   Write a backup copy of the profile store
  completion  Print shell completion script
  doctor   Show setup diagnostics
  version  Show version

Import flags:
  --format openssh|sshdex       Input format (default openssh)
  --conflict skip|replace|rename  Conflict policy (default skip)
  --dry-run                      Preview import plan without writing

Global flags:
  --store PATH  Override profile store path`)
}

package app

import (
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
	"github.com/bbrandlegacy/sshdex/internal/sshcmd"
	"github.com/bbrandlegacy/sshdex/internal/sshconfig"
	"github.com/bbrandlegacy/sshdex/internal/store"
)

const Version = "0.2.0"

type options struct {
	storePath string
	args      []string
}

func Run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseGlobal(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
	case "import":
		return runImport(opts.storePath, rest, stdout, stderr)
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

func runAdd(path string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	p, tags, locals, remotes, dynamics := bindProfileFlags(fs, profile.Profile{})
	if err := fs.Parse(args); err != nil {
		return 2
	}
	p.Tags = tags.Values
	p.LocalForwards = locals.Values
	p.RemoteForwards = remotes.Values
	p.DynamicForwards = dynamics.Values
	s, err := store.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	if err := s.Add(*p); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := s.Save(); err != nil {
		fmt.Fprintln(stderr, err)
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
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, p := range s.List() {
		if *tag != "" && !hasTag(p, *tag) {
			continue
		}
		if *search != "" && !matchesSearch(p, *search) {
			continue
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", p.Name, p.Host, p.User, strings.Join(p.Tags, ","))
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
		fmt.Fprintln(stderr, err)
		return 1
	}
	p, err := s.Get(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Name: %s\nHost: %s\nUser: %s\nPort: %d\nIdentityFile: %s\nProxyJump: %s\nLocalForwards: %s\nRemoteForwards: %s\nDynamicForwards: %s\nRemoteCommand: %s\nTags: %s\nNotes: %s\nConnectionCount: %d\n", p.Name, p.Host, p.User, p.Port, p.IdentityFile, p.ProxyJump, strings.Join(p.LocalForwards, ","), strings.Join(p.RemoteForwards, ","), strings.Join(p.DynamicForwards, ","), p.RemoteCommand, strings.Join(p.Tags, ","), p.Notes, p.ConnectionCount)
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
		fmt.Fprintln(stderr, err)
		return 1
	}
	existing, err := s.Get(name)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	p, tags, locals, remotes, dynamics := bindProfileFlags(fs, existing)
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
	p.Name = existing.Name
	p.CreatedAt = existing.CreatedAt
	p.UpdatedAt = time.Now().UTC()
	p.LastConnectedAt = existing.LastConnectedAt
	p.ConnectionCount = existing.ConnectionCount
	if err := s.Update(*p); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := s.Save(); err != nil {
		fmt.Fprintln(stderr, err)
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
		fmt.Fprintln(stderr, "delete requires --force in v0.1")
		return 2
	}
	s, err := store.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := s.Delete(names[0]); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := s.Save(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "deleted %s\n", names[0])
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
		fmt.Fprintln(stderr, err)
		return 1
	}
	p, err := s.Get(names[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if dryRun {
		preview, err := sshcmd.Preview(p)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, preview)
		return 0
	}
	argv, err := sshcmd.BuildArgs(p)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runDoctor(path string, stdout, stderr io.Writer) int {
	s, err := store.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sshPath, err := exec.LookPath("ssh")
	sshStatus := "missing"
	if err == nil {
		sshStatus = sshPath
	}
	invalid := doctorInvalidProfiles(s.List())
	fmt.Fprintf(stdout, "Store: %s\nProfiles: %d\nSSH: %s\nInvalidProfiles: %d\n", path, len(s.List()), sshStatus, len(invalid))
	for _, msg := range invalid {
		fmt.Fprintf(stdout, "- %s\n", msg)
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
	dryRun := false
	files := []string{}
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		default:
			files = append(files, arg)
		}
	}
	if len(files) != 1 {
		fmt.Fprintln(stderr, "import requires PATH")
		return 2
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	profiles, err := sshconfig.ParseString(string(data))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if dryRun {
		for _, p := range profiles {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", p.Name, p.Host, p.User)
		}
		return 0
	}
	s, err := store.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	imported, skipped := 0, 0
	for _, p := range profiles {
		if err := s.Add(p); err != nil {
			if errors.Is(err, store.ErrProfileExists) {
				skipped++
				continue
			}
			fmt.Fprintln(stderr, err)
			return 1
		}
		imported++
	}
	if err := s.Save(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "imported %d profile(s), skipped %d duplicate(s)\n", imported, skipped)
	return 0
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
  import   Import profiles from an SSH config file
  doctor   Show setup diagnostics
  version  Show version

Global flags:
  --store PATH  Override profile store path`)
}

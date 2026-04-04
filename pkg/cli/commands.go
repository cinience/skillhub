package cli

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Login prompts for registry URL and token, verifies, and saves config.
func Login(args []string) {
	reader := bufio.NewReader(os.Stdin)

	cfg, _ := LoadConfig()

	fmt.Printf("Registry URL [%s]: ", cfg.Registry)
	registry, _ := reader.ReadString('\n')
	registry = strings.TrimSpace(registry)
	if registry != "" {
		cfg.Registry = registry
	}

	fmt.Print("API Token: ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)
	if token == "" {
		exitWithError("token is required")
	}
	cfg.Token = token

	// Verify token
	client := NewClient(cfg)
	user, err := client.WhoAmI()
	if err != nil {
		exitWithError(fmt.Sprintf("authentication failed: %v", err))
	}

	if err := SaveConfig(cfg); err != nil {
		exitWithError(fmt.Sprintf("saving config: %v", err))
	}

	printSuccess(fmt.Sprintf("Logged in as %s (%s)", getStr(user, "handle"), getStr(user, "role")))
}

// WhoAmI shows the currently authenticated user.
func WhoAmI(args []string) {
	client, err := NewClientFromConfig()
	if err != nil {
		exitWithError(err.Error())
	}

	user, err := client.WhoAmI()
	if err != nil {
		exitWithError(err.Error())
	}

	printHeader("Authenticated User")
	printField("Handle", getStr(user, "handle"))
	printField("Role", getStr(user, "role"))
	if email := getStr(user, "email"); email != "" {
		printField("Email", email)
	}
	if name := getStr(user, "displayName"); name != "" {
		printField("Display Name", name)
	}
}

// Search searches for skills by query.
func Search(args []string) {
	if len(args) == 0 {
		exitWithError("Usage: skillhub search <query>")
	}
	query := strings.Join(args, " ")

	client, err := NewClientFromConfig()
	if err != nil {
		exitWithError(err.Error())
	}

	result, err := client.Search(query)
	if err != nil {
		exitWithError(err.Error())
	}

	hits := getSlice(result, "hits")
	if len(hits) == 0 {
		fmt.Println("No results found.")
		return
	}

	headers := []string{"SLUG", "NAME", "SUMMARY", "DOWNLOADS"}
	widths := []int{25, 25, 40, 10}
	var rows [][]string
	for _, hit := range hits {
		m, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}
		rows = append(rows, []string{
			getStr(m, "slug"),
			getStr(m, "displayName"),
			getStr(m, "summary"),
			getNum(m, "downloads"),
		})
	}
	printTable(headers, widths, rows)
}

// List shows skills from the registry.
func List(args []string) {
	sort := "downloads"
	for i, a := range args {
		if a == "--sort" && i+1 < len(args) {
			sort = args[i+1]
		}
	}

	client, err := NewClientFromConfig()
	if err != nil {
		exitWithError(err.Error())
	}

	result, err := client.ListSkills(sort, 20)
	if err != nil {
		exitWithError(err.Error())
	}

	skills := getSlice(result, "data")
	if len(skills) == 0 {
		fmt.Println("No skills found.")
		return
	}

	headers := []string{"SLUG", "NAME", "SUMMARY", "DOWNLOADS", "STARS"}
	widths := []int{25, 25, 35, 10, 6}
	var rows [][]string
	for _, s := range skills {
		m, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		rows = append(rows, []string{
			getStr(m, "slug"),
			getStr(m, "displayName"),
			getStr(m, "summary"),
			getNum(m, "downloads"),
			getNum(m, "starsCount"),
		})
	}
	printTable(headers, widths, rows)
}

// Inspect shows detailed information about a skill.
func Inspect(args []string) {
	if len(args) == 0 {
		exitWithError("Usage: skillhub inspect <slug>")
	}
	slug := args[0]

	client, err := NewClientFromConfig()
	if err != nil {
		exitWithError(err.Error())
	}

	skill, err := client.GetSkill(slug)
	if err != nil {
		exitWithError(err.Error())
	}

	versions, _ := client.GetVersions(slug)

	printHeader(getStr(skill, "slug"))
	if name := getStr(skill, "displayName"); name != "" {
		printField("Name", name)
	}
	if summary := getStr(skill, "summary"); summary != "" {
		printField("Summary", summary)
	}
	printField("Owner", getStr(skill, "ownerHandle"))
	printField("Downloads", getNum(skill, "downloads"))
	printField("Stars", getNum(skill, "starsCount"))
	printField("Versions", getNum(skill, "versionsCount"))
	printField("Status", getStr(skill, "moderationStatus"))

	if tags, ok := skill["tags"]; ok {
		if tagSlice, ok := tags.([]interface{}); ok && len(tagSlice) > 0 {
			var tagStrs []string
			for _, t := range tagSlice {
				tagStrs = append(tagStrs, fmt.Sprintf("%v", t))
			}
			printField("Tags", strings.Join(tagStrs, ", "))
		}
	}

	printField("Created", getStr(skill, "createdAt"))

	if versions != nil {
		if vList := getSlice(versions, "data"); len(vList) > 0 {
			fmt.Println()
			printHeader("Versions")
			headers := []string{"VERSION", "FINGERPRINT", "CREATED"}
			widths := []int{12, 20, 25}
			var rows [][]string
			for _, v := range vList {
				m, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				fp := getStr(m, "fingerprint")
				if len(fp) > 18 {
					fp = fp[:18] + "…"
				}
				rows = append(rows, []string{
					getStr(m, "version"),
					fp,
					getStr(m, "createdAt"),
				})
			}
			printTable(headers, widths, rows)
		}
	}
}

// Install downloads and installs a skill locally.
func Install(args []string) {
	if len(args) == 0 {
		exitWithError("Usage: skillhub install <slug> [--version <version>]")
	}

	slug := args[0]
	version := "latest"
	for i, a := range args {
		if a == "--version" && i+1 < len(args) {
			version = args[i+1]
		}
	}

	client, err := NewClientFromConfig()
	if err != nil {
		exitWithError(err.Error())
	}

	fmt.Printf("Downloading %s@%s...\n", slug, version)

	body, err := client.Download(slug, version)
	if err != nil {
		exitWithError(err.Error())
	}
	defer body.Close()

	// Read entire ZIP into memory
	zipData, err := io.ReadAll(body)
	if err != nil {
		exitWithError(fmt.Sprintf("reading download: %v", err))
	}

	// Create skill directory
	skillDir := filepath.Join(loadSkillsDir(), slug)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		exitWithError(fmt.Sprintf("creating directory: %v", err))
	}

	// Clear existing files (except .installed.json)
	clearSkillDir(skillDir)

	// Extract ZIP
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		exitWithError(fmt.Sprintf("reading zip: %v", err))
	}

	// Detect common top-level directory prefix to strip
	stripPrefix := detectZipPrefix(zipReader)

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		name := f.Name
		// Strip the common top-level directory
		if stripPrefix != "" {
			name = strings.TrimPrefix(name, stripPrefix)
			if name == "" {
				continue
			}
		}

		targetPath := filepath.Join(skillDir, name)

		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(skillDir)+string(os.PathSeparator)) {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			exitWithError(fmt.Sprintf("creating directory: %v", err))
		}

		rc, err := f.Open()
		if err != nil {
			exitWithError(fmt.Sprintf("opening zip entry: %v", err))
		}

		outFile, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			exitWithError(fmt.Sprintf("creating file: %v", err))
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			outFile.Close()
			rc.Close()
			exitWithError(fmt.Sprintf("extracting file: %v", err))
		}
		outFile.Close()
		rc.Close()
	}

	// Write install metadata
	meta := map[string]interface{}{
		"version":     version,
		"installedAt": time.Now().UTC().Format(time.RFC3339),
		"slug":        slug,
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(skillDir, ".installed.json"), metaData, 0644)

	printSuccess(fmt.Sprintf("Installed %s@%s to %s", slug, version, skillDir))
}

// Uninstall removes a locally installed skill.
func Uninstall(args []string) {
	if len(args) == 0 {
		exitWithError("Usage: skillhub uninstall <slug>")
	}
	slug := args[0]

	skillDir := filepath.Join(loadSkillsDir(), slug)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		exitWithError(fmt.Sprintf("skill %q is not installed", slug))
	}

	if err := os.RemoveAll(skillDir); err != nil {
		exitWithError(fmt.Sprintf("removing skill: %v", err))
	}

	printSuccess(fmt.Sprintf("Uninstalled %s", slug))
}

// Installed lists locally installed skills.
func Installed(args []string) {
	dir := loadSkillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No skills installed.")
			return
		}
		exitWithError(fmt.Sprintf("reading skills directory: %v", err))
	}

	headers := []string{"SLUG", "VERSION", "INSTALLED AT"}
	widths := []int{30, 15, 25}
	var rows [][]string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(dir, entry.Name(), ".installed.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			rows = append(rows, []string{entry.Name(), "unknown", "unknown"})
			continue
		}
		var meta map[string]interface{}
		json.Unmarshal(data, &meta)
		rows = append(rows, []string{
			entry.Name(),
			getStr(meta, "version"),
			getStr(meta, "installedAt"),
		})
	}

	if len(rows) == 0 {
		fmt.Println("No skills installed.")
		return
	}

	printTable(headers, widths, rows)
}

// Update checks for and downloads newer versions of installed skills.
func Update(args []string) {
	updateAll := false
	var targets []string
	for _, a := range args {
		if a == "--all" {
			updateAll = true
		} else if !strings.HasPrefix(a, "-") {
			targets = append(targets, a)
		}
	}

	dir := loadSkillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No skills installed.")
			return
		}
		exitWithError(fmt.Sprintf("reading skills directory: %v", err))
	}

	client, err := NewClientFromConfig()
	if err != nil {
		exitWithError(err.Error())
	}

	updated := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()

		// Filter by targets if not --all
		if !updateAll && len(targets) > 0 {
			found := false
			for _, t := range targets {
				if t == slug {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Read current installed version
		metaPath := filepath.Join(dir, slug, ".installed.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta map[string]interface{}
		json.Unmarshal(data, &meta)
		currentVer := getStr(meta, "version")

		// Check latest version from registry
		skill, err := client.GetSkill(slug)
		if err != nil {
			fmt.Printf("  %s: failed to check (%v)\n", slug, err)
			continue
		}

		// Get latest version info via versions endpoint
		versions, err := client.GetVersions(slug)
		if err != nil {
			continue
		}
		vList := getSlice(versions, "data")
		if len(vList) == 0 {
			continue
		}

		// First version in list is the latest
		latestMap, ok := vList[0].(map[string]interface{})
		if !ok {
			continue
		}
		latestVer := getStr(latestMap, "version")
		_ = skill

		if latestVer == currentVer {
			fmt.Printf("  %s: up to date (%s)\n", slug, currentVer)
			continue
		}

		fmt.Printf("  %s: %s → %s, updating...\n", slug, currentVer, latestVer)
		Install([]string{slug, "--version", latestVer})
		updated++
	}

	if updated == 0 {
		fmt.Println("All skills are up to date.")
	} else {
		printSuccess(fmt.Sprintf("Updated %d skill(s).", updated))
	}
}

// Publish publishes a skill from a local directory.
func Publish(args []string) {
	if len(args) == 0 {
		exitWithError("Usage: skillhub publish <path> [--slug <slug>] [--version <version>] [--tags <tags>] [--summary <summary>]")
	}

	dirPath := args[0]
	slug := getFlag(args[1:], "--slug")
	version := getFlag(args[1:], "--version")
	tags := getFlag(args[1:], "--tags")
	summary := getFlag(args[1:], "--summary")
	changelog := getFlag(args[1:], "--changelog")

	// Verify directory exists
	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		exitWithError(fmt.Sprintf("%q is not a valid directory", dirPath))
	}

	// Read all files
	files, err := ReadDirFiles(dirPath)
	if err != nil {
		exitWithError(fmt.Sprintf("reading directory: %v", err))
	}

	if len(files) == 0 {
		exitWithError("no files found in directory")
	}

	// Try to extract slug from SKILL.md frontmatter if not specified
	if slug == "" {
		if content, ok := files["SKILL.md"]; ok {
			slug = extractFrontmatterField(string(content), "name")
		}
	}
	if slug == "" {
		exitWithError("--slug is required (or set 'name' in SKILL.md frontmatter)")
	}

	client, err := NewClientFromConfig()
	if err != nil {
		exitWithError(err.Error())
	}

	fmt.Printf("Publishing %s...\n", slug)
	if version != "" {
		fmt.Printf("  Version: %s\n", version)
	}
	fmt.Printf("  Files: %d\n", len(files))

	result, err := client.Publish(slug, version, summary, tags, changelog, files)
	if err != nil {
		exitWithError(err.Error())
	}

	// Extract version info from result
	if vMap, ok := result["version"].(map[string]interface{}); ok {
		printSuccess(fmt.Sprintf("Published %s@%s", slug, getStr(vMap, "version")))
	} else {
		printSuccess(fmt.Sprintf("Published %s", slug))
	}
}

// PrintUsage prints CLI usage help.
func PrintUsage() {
	fmt.Println(`Usage: skillhub <command> [arguments]

Commands:
  serve                         Start the HTTP server
  admin                         Admin operations (create-user, create-token)

  login                         Authenticate with a SkillHub registry
  whoami                        Show current authenticated user

  search <query>                Search for skills
  list [--sort <field>]         List skills from the registry
  inspect <slug>                Show detailed skill information

  install <slug> [--version v]  Install a skill locally
  uninstall <slug>              Remove a locally installed skill
  installed                     List locally installed skills
  update [--all] [slug...]      Update installed skills

  publish <path> [flags]        Publish a skill to the registry
    --slug <slug>               Skill slug (or set 'name' in SKILL.md)
    --version <version>         Semantic version
    --tags <tags>               Comma-separated tags
    --summary <text>            Short description
    --changelog <text>          Changelog for this version

Configuration is stored in ~/.skillhub/config.yaml
Installed skills are stored in ~/.skillhub/skills/ (configurable via skills_dir in config.yaml)`)
}

// --- helpers ---

func loadSkillsDir() string {
	cfg, _ := LoadConfig()
	return SkillsDir(cfg)
}

func getFlag(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func getSlice(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key]; ok {
		if s, ok := v.([]interface{}); ok {
			return s
		}
	}
	return nil
}

func clearSkillDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() == ".installed.json" {
			continue
		}
		os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

// extractFrontmatterField extracts a field from YAML frontmatter in a markdown file.
func extractFrontmatterField(content, field string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return ""
	}
	frontmatter := content[3 : 3+end]
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+":") {
			val := strings.TrimPrefix(line, field+":")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, "\"'")
			return val
		}
	}
	return ""
}

// detectZipPrefix finds a common top-level directory in a ZIP archive.
// If all files start with "somedir/", returns "somedir/".
// Returns "" if there's no common prefix.
func detectZipPrefix(zr *zip.Reader) string {
	if len(zr.File) == 0 {
		return ""
	}

	// Find the first path component of the first non-directory entry
	var prefix string
	for _, f := range zr.File {
		name := f.Name
		if idx := strings.Index(name, "/"); idx > 0 {
			prefix = name[:idx+1]
			break
		}
	}
	if prefix == "" {
		return ""
	}

	// Check all entries share this prefix
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, prefix) {
			return ""
		}
	}
	return prefix
}

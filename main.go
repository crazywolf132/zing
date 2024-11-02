package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	openai "github.com/sashabaranov/go-openai"
)

type Config struct {
	AI       AIConfig       `toml:"ai"`
	Commit   CommitConfig   `toml:"commit"`
	System   SystemConfig   `toml:"system"`
	Display  DisplayConfig  `toml:"display"`
	Template TemplateConfig `toml:"template"`
}

type AIConfig struct {
	Provider    string  `toml:"provider"` // "openai" or "ollama"
	Model       string  `toml:"model"`
	MaxTokens   int     `toml:"max_tokens"`
	Temperature float32 `toml:"temperature"`

	Ollama struct {
		URL string `toml:"url"`
	} `toml:"ollama"`
}

type CommitConfig struct {
	Style              string   `toml:"style"`        // "conventional" or "detailed" or "custom"
	IncludeScope       bool     `toml:"scope"`        // Include scope in conventional commits
	IncludeBreaking    bool     `toml:"breaking"`     // Include breaking changes section
	MaxLength          int      `toml:"max_length"`   // Maximum length of commit message
	ScopePrefix        []string `toml:"scope_prefix"` // Allowed scope prefixes
	JiraIntegration    bool     `toml:"jira"`         // Include JIRA ticket from branch name
	CoAuthors          []string `toml:"co_authors"`   // List of co-authors to include
	SignCommits        bool     `toml:"sign"`         // GPG sign commits
	EmojisEnabled      bool     `toml:"emojis"`       // Use emojis in commits
	VerifyConventional bool     `toml:"verify"`       // Verify conventional commit format
}

type SystemConfig struct {
	MaxRetries     int      `toml:"max_retries"`
	RetryDelay     int      `toml:"retry_delay"`      // seconds
	Timeout        int      `toml:"timeout"`          // seconds
	MaxDiffSize    int      `toml:"max_diff_size"`    // bytes
	MaxConcurrent  int      `toml:"max_concurrent"`   // max concurrent API calls
	MaxMessageSize int      `toml:"max_message_size"` // bytes
	GitHooksPath   string   `toml:"git_hooks_path"`   // Path to git hooks
	CachePath      string   `toml:"cache_path"`       // Path to cache directory
	IgnorePaths    []string `toml:"ignore_paths"`     // Paths to ignore in diff
}

type DisplayConfig struct {
	Debug      bool   `toml:"debug"`
	ColorMode  string `toml:"color_mode"` // "auto", "always", "never"
	ShowDiff   bool   `toml:"show_diff"`  // Show diff in confirmation
	Quiet      bool   `toml:"quiet"`      // Minimal output
	TimeFormat string `toml:"time_format"`
	DiffFormat string `toml:"diff_format"` // "unified", "minimal", "patience"
}

type TemplateConfig struct {
	CustomTemplates map[string]string `toml:"custom_templates"`
	ActiveTemplate  string            `toml:"active_template"`
}

type OllamaRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type GitInfo struct {
	Files        []FileChange
	Branch       string
	JiraTicket   string
	LastCommit   string
	TotalChanges struct {
		Additions int
		Deletions int
	}
}

type FileChange struct {
	Path     string
	Status   string // Added, Modified, Deleted, Renamed
	Addition int    // Lines added
	Deletion int    // Lines deleted
	IsBinary bool
	Diff     string
	Language string // Detected programming language
}

var (
	configFile string
	config     Config
	debug      *color.Color
	info       *color.Color
	warn       *color.Color
	error_     *color.Color
	cache      *CommitCache
)

type CommitCache struct {
	Path    string
	Records map[string]CommitRecord
}

type CommitRecord struct {
	Message   string    `json:"message"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}

func init() {
	// Initialize colored output with respect to config and terminal capabilities
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		color.NoColor = true
	}

	debug = color.New(color.FgCyan)
	info = color.New(color.FgGreen)
	warn = color.New(color.FgYellow)
	error_ = color.New(color.FgRed)

	home, err := os.UserHomeDir()
	if err != nil {
		error_.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	// Set default config file location based on OS
	switch runtime.GOOS {
	case "darwin", "linux":
		configFile = filepath.Join(home, ".config", "zing", "config.toml")
	case "windows":
		configFile = filepath.Join(os.Getenv("APPDATA"), "zing", "config.toml")
	default:
		error_.Fprintf(os.Stderr, "Unsupported operating system: %s\n", runtime.GOOS)
		os.Exit(1)
	}

	// Initialize directories
	for _, dir := range []string{
		filepath.Dir(configFile),
		filepath.Join(home, ".cache", "zing"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			error_.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	// Initialize cache
	cache = &CommitCache{
		Path:    filepath.Join(home, ".cache", "zing", "commits.json"),
		Records: make(map[string]CommitRecord),
	}
	if err := cache.Load(); err != nil {
		warn.Printf("Could not load commit cache: %v\n", err)
	}

	// Load or create default config
	if err := loadConfig(); err != nil {
		error_.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Apply color mode setting
	switch config.Display.ColorMode {
	case "always":
		color.NoColor = false
	case "never":
		color.NoColor = true
	}
}

func (c *CommitCache) Load() error {
	data, err := os.ReadFile(c.Path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &c.Records)
}

func (c *CommitCache) Save() error {
	data, err := json.MarshalIndent(c.Records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.Path, data, 0644)
}

func (c *CommitCache) Add(message string, hash string, success bool) {
	c.Records[hash] = CommitRecord{
		Message:   message,
		Hash:      hash,
		Timestamp: time.Now(),
		Success:   success,
	}
	c.Save()
}

func loadConfig() error {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		defaultConfig := Config{
			AI: AIConfig{
				Provider:    "ollama",
				Model:       "llama2",
				MaxTokens:   500,
				Temperature: 0.7,
				Ollama: struct {
					URL string `toml:"url"`
				}{
					URL: "http://localhost:11434/api/chat",
				},
			},
			Commit: CommitConfig{
				Style:              "conventional",
				IncludeScope:       true,
				IncludeBreaking:    true,
				MaxLength:          72,
				ScopePrefix:        []string{"feat", "fix", "docs", "style", "refactor", "test", "chore"},
				JiraIntegration:    true,
				SignCommits:        false,
				EmojisEnabled:      false,
				VerifyConventional: true,
			},
			System: SystemConfig{
				MaxRetries:     3,
				RetryDelay:     2,
				Timeout:        30,
				MaxDiffSize:    1024 * 1024,
				MaxConcurrent:  4,
				MaxMessageSize: 4096,
				GitHooksPath:   ".git/hooks",
				CachePath:      filepath.Join(os.TempDir(), "zing"),
				IgnorePaths:    []string{".env", "*.lock", "node_modules/"},
			},
			Display: DisplayConfig{
				Debug:      false,
				ColorMode:  "auto",
				ShowDiff:   true,
				Quiet:      false,
				TimeFormat: "2006-01-02 15:04:05",
				DiffFormat: "unified",
			},
			Template: TemplateConfig{
				CustomTemplates: map[string]string{
					"default": "{{.Type}}{{if .Scope}}({{.Scope}}){{end}}: {{.Description}}",
					"detailed": `{{.Type}}{{if .Scope}}({{.Scope}}){{end}}: {{.Description}}

{{.Body}}

{{if .Breaking}}BREAKING CHANGE: {{.Breaking}}{{end}}
{{if .Closes}}Closes: {{.Closes}}{{end}}`,
				},
				ActiveTemplate: "default",
			},
		}

		file, err := os.Create(configFile)
		if err != nil {
			return fmt.Errorf("error creating config file: %w", err)
		}
		defer file.Close()

		encoder := toml.NewEncoder(file)
		if err := encoder.Encode(defaultConfig); err != nil {
			return fmt.Errorf("error writing default config: %w", err)
		}

		config = defaultConfig
		return nil
	}

	_, err := toml.DecodeFile(configFile, &config)
	return err
}

func detectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".go":
		return "Go"
	case ".js", ".jsx":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".py":
		return "Python"
	case ".rb":
		return "Ruby"
	case ".java":
		return "Java"
	case ".php":
		return "PHP"
	case ".rs":
		return "Rust"
	case ".c":
		return "C"
	case ".cpp":
		return "C++"
	case ".cs":
		return "C#"
	case ".html":
		return "HTML"
	case ".css":
		return "CSS"
	case ".md":
		return "Markdown"
	default:
		return "Unknown"
	}
}

func getGitInfo() (*GitInfo, error) {
	gitInfo := &GitInfo{}

	// Get current branch
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchOutput, err := branchCmd.Output()
	if err == nil {
		gitInfo.Branch = strings.TrimSpace(string(branchOutput))
		// Extract JIRA ticket if enabled
		if config.Commit.JiraIntegration {
			re := regexp.MustCompile(`[A-Z]+-\d+`)
			if match := re.FindString(gitInfo.Branch); match != "" {
				gitInfo.JiraTicket = match
			}
		}
	}

	// Get last commit hash
	hashCmd := exec.Command("git", "rev-parse", "HEAD")
	hashOutput, err := hashCmd.Output()
	if err == nil {
		gitInfo.LastCommit = strings.TrimSpace(string(hashOutput))
	}

	// Get staged files
	cmd := exec.Command("git", "diff", "--cached", "--name-status")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting staged files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, file := range files {
		if file == "" {
			continue
		}

		parts := strings.Fields(file)
		if len(parts) < 2 {
			continue
		}

		status := parts[0]
		path := parts[1]

		// Check if path should be ignored
		ignored := false
		for _, pattern := range config.System.IgnorePaths {
			if matched, _ := filepath.Match(pattern, path); matched {
				ignored = true
				break
			}
		}
		if ignored {
			continue
		}

		// Get file diff
		diff, err := getFileDiff(path)
		if err != nil {
			warn.Printf("Warning: Could not get diff for %s: %v\n", path, err)
			continue
		}

		// Check if file is binary
		cmd = exec.Command("git", "diff", "--cached", "--numstat", path)
		stats, err := cmd.Output()
		if err != nil {
			warn.Printf("Warning: Could not get stats for %s: %v\n", path, err)
			continue
		}

		statsFields := strings.Fields(string(stats))
		isBinary := len(statsFields) >= 2 && statsFields[0] == "-" && statsFields[1] == "-"

		fileChange := FileChange{
			Path:     path,
			Status:   parseGitStatus(status),
			IsBinary: isBinary,
			Diff:     diff,
			Language: detectLanguage(path),
		}

		if !isBinary && len(statsFields) >= 2 {
			fileChange.Addition, _ = strconv.Atoi(statsFields[0])
			fileChange.Deletion, _ = strconv.Atoi(statsFields[1])
			gitInfo.TotalChanges.Additions += fileChange.Addition
			gitInfo.TotalChanges.Deletions += fileChange.Deletion
		}

		gitInfo.Files = append(gitInfo.Files, fileChange)
	}

	return gitInfo, nil
}

func parseGitStatus(status string) string {
	switch status[0] {
	case 'A':
		return "Added"
	case 'M':
		return "Modified"
	case 'D':
		return "Deleted"
	case 'R':
		return "Renamed"
	case 'C':
		return "Copied"
	case 'U':
		return "Unmerged"
	default:
		return "Unknown"
	}
}

func getFileDiff(file string) (string, error) {
	var args []string
	switch config.Display.DiffFormat {
	case "minimal":
		args = []string{"diff", "--cached", "--minimal"}
	case "patience":
		args = []string{"diff", "--cached", "--patience"}
	default:
		args = []string{"diff", "--cached"}
	}
	args = append(args, file)

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting file diff: %w", err)
	}
	return string(output), nil
}

type CommitTemplateData struct {
	Type        string
	Scope       string
	Description string
	Body        string
	Breaking    string
	Closes      string
	JiraTicket  string
	CoAuthors   []string
}

func generateCommitMessage(gitInfo *GitInfo) (string, error) {
	var prompt strings.Builder

	prompt.WriteString("Generate a commit message for the following changes:\n\n")
	prompt.WriteString(fmt.Sprintf("Total Changes: +%d/-%d lines\n",
		gitInfo.TotalChanges.Additions,
		gitInfo.TotalChanges.Deletions))

	// Add contextual information
	prompt.WriteString(fmt.Sprintf("\nBranch: %s\n", gitInfo.Branch))
	if gitInfo.JiraTicket != "" {
		prompt.WriteString(fmt.Sprintf("JIRA Ticket: %s\n", gitInfo.JiraTicket))
	}

	// Add language-specific context
	languageStats := make(map[string]int)
	for _, file := range gitInfo.Files {
		languageStats[file.Language]++
	}
	prompt.WriteString("\nLanguages affected:\n")
	for lang, count := range languageStats {
		prompt.WriteString(fmt.Sprintf("- %s (%d files)\n", lang, count))
	}

	// Add file changes
	prompt.WriteString("\nChanged files:\n")
	for _, file := range gitInfo.Files {
		prompt.WriteString(fmt.Sprintf("\n=== %s (%s) ===\n", file.Path, file.Status))
		if !file.IsBinary {
			prompt.WriteString(fmt.Sprintf("Changes: +%d/-%d lines\n", file.Addition, file.Deletion))
			prompt.WriteString(file.Diff)
		} else {
			prompt.WriteString("[Binary file]\n")
		}
	}

	// Add style instructions
	prompt.WriteString("\nPlease generate a commit message following these rules:\n")
	if config.Commit.Style == "conventional" {
		prompt.WriteString(`
1. Use conventional commit format: <type>(<scope>): <description>
2. Types should be one of: ` + strings.Join(config.Commit.ScopePrefix, ", ") + `
3. Keep the description concise and clear
4. Use imperative mood ("add" not "added")`)
		if config.Commit.IncludeBreaking {
			prompt.WriteString("\n5. If there are breaking changes, include a BREAKING CHANGE section")
		}
	} else if config.Commit.Style == "detailed" {
		prompt.WriteString(`
1. Start with a clear summary line
2. Add a detailed body explaining the changes
3. Include technical details where relevant
4. Mention any potential side effects`)
	}

	debugLog("Generated prompt:\n%s", prompt.String())

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.System.Timeout)*time.Second)
	defer cancel()

	var message string
	var err error

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Generating commit message..."
	s.Start()
	defer s.Stop()

	// Try generating message with retries
	for attempt := 1; attempt <= config.System.MaxRetries; attempt++ {
		switch config.AI.Provider {
		case "openai":
			message, err = generateWithOpenAI(ctx, prompt.String())
		case "ollama":
			message, err = generateWithOllama(ctx, prompt.String())
		default:
			return "", fmt.Errorf("unsupported provider: %s", config.AI.Provider)
		}

		if err == nil {
			break
		}

		if attempt == config.System.MaxRetries {
			return "", fmt.Errorf("failed after %d attempts: %w", config.System.MaxRetries, err)
		}

		warn.Printf("Attempt %d failed: %v. Retrying in %d seconds...\n",
			attempt, err, config.System.RetryDelay)
		time.Sleep(time.Duration(config.System.RetryDelay) * time.Second)
	}

	// Post-process the message
	message = postProcessCommitMessage(message, gitInfo)

	// Verify conventional commit format if enabled
	if config.Commit.VerifyConventional && config.Commit.Style == "conventional" {
		if err := verifyConventionalCommit(message); err != nil {
			return "", fmt.Errorf("generated message does not follow conventional commit format: %w", err)
		}
	}

	return message, nil
}

func postProcessCommitMessage(message string, gitInfo *GitInfo) string {
	// Add JIRA ticket if enabled and not already present
	if config.Commit.JiraIntegration && gitInfo.JiraTicket != "" {
		if !strings.Contains(message, gitInfo.JiraTicket) {
			message = fmt.Sprintf("%s [%s]", message, gitInfo.JiraTicket)
		}
	}

	// Add co-authors if configured
	if len(config.Commit.CoAuthors) > 0 {
		message += "\n\n"
		for _, author := range config.Commit.CoAuthors {
			message += fmt.Sprintf("Co-authored-by: %s\n", author)
		}
	}

	// Add emojis if enabled
	if config.Commit.EmojisEnabled {
		message = addCommitEmojis(message)
	}

	// Ensure message isn't too long
	if len(message) > config.Commit.MaxLength {
		lines := strings.Split(message, "\n")
		lines[0] = lines[0][:config.Commit.MaxLength]
		message = strings.Join(lines, "\n")
	}

	return message
}

func verifyConventionalCommit(message string) error {
	pattern := `^(?i)(` + strings.Join(config.Commit.ScopePrefix, "|") + `)`
	if config.Commit.IncludeScope {
		pattern += `(\([^)]+\))?`
	}
	pattern += `: .+`

	matched, err := regexp.MatchString(pattern, message)
	if err != nil {
		return fmt.Errorf("error matching pattern: %w", err)
	}
	if !matched {
		return fmt.Errorf("message does not match conventional commit format")
	}
	return nil
}

func addCommitEmojis(message string) string {
	emojiMap := map[string]string{
		"feat":     "✨",
		"fix":      "🐛",
		"docs":     "📚",
		"style":    "💎",
		"refactor": "♻️",
		"test":     "🧪",
		"chore":    "🔧",
	}

	for typeStr, emoji := range emojiMap {
		pattern := fmt.Sprintf(`^%s(\([^)]+\))?:`, typeStr)
		re := regexp.MustCompile(pattern)
		message = re.ReplaceAllString(message, fmt.Sprintf("%s %s$0", emoji, typeStr))
	}

	return message
}

func generateWithOpenAI(ctx context.Context, prompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	client := openai.NewClient(apiKey)
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: config.AI.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			MaxTokens:   config.AI.MaxTokens,
			Temperature: config.AI.Temperature,
		},
	)

	if err != nil {
		return "", fmt.Errorf("error generating with OpenAI: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func generateWithOllama(ctx context.Context, prompt string) (string, error) {
	reqBody := OllamaRequest{
		Model: config.AI.Model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: config.AI.Temperature,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.AI.Ollama.URL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	return ollamaResp.Message.Content, nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "zing",
		Short: "AI-powered commit message generator",
		Long: `Zing is a smart commit message generator that uses AI to create
meaningful commit messages based on your staged changes.

It supports both OpenAI and Ollama as AI providers and can generate
messages in conventional commits format or detailed style.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if we're in a git repository
			if _, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err != nil {
				return fmt.Errorf("not a git repository")
			}

			gitInfo, err := getGitInfo()
			if err != nil {
				return err
			}

			if len(gitInfo.Files) == 0 {
				return fmt.Errorf("no staged changes found")
			}

			if !config.Display.Quiet {
				info.Printf("Found %d staged files", len(gitInfo.Files))
				fmt.Println("Changes summary:")
				for _, file := range gitInfo.Files {
					if file.IsBinary {
						fmt.Printf("  %s: %s (binary file)\n", file.Status, file.Path)
					} else {
						fmt.Printf("  %s: %s (+%d/-%d)\n", file.Status, file.Path, file.Addition, file.Deletion)
					}
				}
			}

			message, err := generateCommitMessage(gitInfo)
			if err != nil {
				return fmt.Errorf("error generating commit message: %w", err)
			}

			autoConfirm, _ := cmd.Flags().GetBool("yes")
			if !autoConfirm {
				fmt.Printf("\nGenerated commit message:\n%s\n\n", message)
				if config.Display.ShowDiff {
					fmt.Println("Changes to be committed:")
					diffCmd := exec.Command("git", "diff", "--cached", "--color")
					diffCmd.Stdout = os.Stdout
					diffCmd.Run()
				}
				fmt.Print("Proceed with commit? [Y/n] ")
				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))
				if response == "n" || response == "no" {
					fmt.Println("commit cancelled by user")
					return nil
				}
			}

			// Prepare commit command
			args = []string{"commit", "-m", message}
			if config.Commit.SignCommits {
				args = append(args, "-S")
			}

			// Execute git commit
			commitCmd := exec.Command("git", args...)
			commitCmd.Stdout = os.Stdout
			commitCmd.Stderr = os.Stderr
			if err := commitCmd.Run(); err != nil {
				return fmt.Errorf("error executing git commit: %w", err)
			}

			// Get the commit hash
			hashCmd := exec.Command("git", "rev-parse", "HEAD")
			hashOutput, err := hashCmd.Output()
			if err == nil {
				hash := strings.TrimSpace(string(hashOutput))
				cache.Add(message, hash, true)
			}

			if !config.Display.Quiet {
				info.Println("Successfully committed changes!")
			}
			return nil
		},
	}

	// Add flags
	rootCmd.Flags().BoolP("yes", "y", false, "Automatically confirm and proceed with commit")
	rootCmd.Flags().StringP("template", "t", "", "Use specific commit message template")
	rootCmd.Flags().BoolP("version", "v", false, "Show version information")

	// Config command
	var configCmd = &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  "View or modify Zing configuration settings",
	}

	// Show config
	var showConfigCmd = &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Config file location: %s\n\n", configFile)
			fmt.Printf("Current configuration:\n")
			encoder := toml.NewEncoder(os.Stdout)
			encoder.Encode(config)
		},
	}

	// Edit config
	var editConfigCmd = &cobra.Command{
		Use:   "edit",
		Short: "Open configuration file in default editor",
		Run: func(cmd *cobra.Command, args []string) {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}

			editCmd := exec.Command(editor, configFile)
			editCmd.Stdin = os.Stdin
			editCmd.Stdout = os.Stdout
			editCmd.Stderr = os.Stderr

			if err := editCmd.Run(); err != nil {
				error_.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
				os.Exit(1)
			}

			if err := loadConfig(); err != nil {
				error_.Fprintf(os.Stderr, "Error reloading config: %v\n", err)
				os.Exit(1)
			}
			info.Println("Configuration reloaded successfully")
		},
	}

	// Add template command
	var templateCmd = &cobra.Command{
		Use:   "template",
		Short: "Manage commit message templates",
	}

	var addTemplateCmd = &cobra.Command{
		Use:   "add [name] [template]",
		Short: "Add a new commit message template",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			templateStr := args[1]

			// Validate template
			_, err := template.New(name).Parse(templateStr)
			if err != nil {
				error_.Fprintf(os.Stderr, "Invalid template: %v\n", err)
				os.Exit(1)
			}

			config.Template.CustomTemplates[name] = templateStr
			if err := saveConfig(); err != nil {
				error_.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}
			info.Printf("Template '%s' added successfully\n", name)
		},
	}

	// Add commands
	templateCmd.AddCommand(addTemplateCmd)
	configCmd.AddCommand(showConfigCmd, editConfigCmd)
	rootCmd.AddCommand(configCmd, templateCmd)

	// Initialize hooks command
	var hooksCmd = &cobra.Command{
		Use:   "hooks",
		Short: "Manage git hooks",
		Run: func(cmd *cobra.Command, args []string) {
			if err := installGitHooks(); err != nil {
				error_.Fprintf(os.Stderr, "Error installing git hooks: %v\n", err)
				os.Exit(1)
			}
			info.Println("Git hooks installed successfully")
		},
	}

	rootCmd.AddCommand(hooksCmd)

	if err := rootCmd.Execute(); err != nil {
		error_.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func installGitHooks() error {
	hookContent := `#!/bin/sh
# Zing pre-commit hook
zing --yes`

	hookPath := filepath.Join(config.System.GitHooksPath, "prepare-commit-msg")
	return os.WriteFile(hookPath, []byte(hookContent), 0755)
}

func saveConfig() error {
	file, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("error creating config file: %w", err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(config)
}

func debugLog(format string, args ...interface{}) {
	if config.Display.Debug {
		debug.Printf("[DEBUG] "+format+"\n", args...)
	}
}

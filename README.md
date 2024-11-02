# 🎉 Zing: Give Your Git Commits Some Zing! 🎉

*Because every commit deserves a little pizzazz!*

---

## 🚀 Introduction

**Zing** is the ultimate sidekick for developers who are tired of writing mundane commit messages. Let AI handle the grunt work while you focus on what you do best: coding!

Imagine a world where your commit messages are not just informative but also stylishly adhere to the Conventional Commits standard—all without lifting a finger. That's the magic of Zing!

---

## ✨ Features

- **🧠 AI-Powered Commit Messages**: Leverage the brilliance of OpenAI or Ollama to generate meaningful commit messages.
- **⚙️ Customizable AI Providers**: Choose your AI adventure—use OpenAI, Ollama, or both!
- **📏 Smart Diff Management**: Automatically handles large diffs to keep your commits efficient.
- **💰 Cost Control**: Set token limits to keep your AI usage (and expenses) in check.
- **📝 Configurable Prompts**: Provide examples to guide the AI in crafting the perfect message.
- **🔒 Secure**: Reads OpenAI API keys from environment variables to keep your secrets safe.
- **🎯 Intuitive and Innovative**: Designed to be simple yet powerful—a tool that truly understands developers.

---

## 🎩 Why Zing?

Let's face it—writing commit messages can be a drag. Whether you're racing against a deadline or just not feeling inspired, Zing has got your back. Here's why you'll love it:

- **Save Time**: Let AI do the heavy lifting so you can get back to coding.
- **Stay Consistent**: Automatically generate commit messages that follow the Conventional Commits standard.
- **Boost Productivity**: Streamline your workflow with intelligent automation.
- **Have Fun**: Turn a tedious task into something enjoyable!

---

## 🛠 Installation

### Prerequisites

- **Go (Golang)**: [Install Go](https://golang.org/dl/)
- **Git**: [Install Git](https://git-scm.com/downloads)
- **OpenAI API Key** (if using OpenAI): [Get an API key](https://beta.openai.com/signup/)

### Get Zing Up and Running

1. **Clone the Zing Repository**

   ```bash
   git clone https://github.com/crazywolf132/zing.git
   cd zing
   ```

2. **Build the Zing Executable**

   ```bash
   go build -o zing
   ```

3. **Make It Global (Optional but Awesome)**

   ```bash
   sudo mv zing /usr/local/bin/
   ```

4. **Configure Your Environment**

   Set your OpenAI API key:

   ```bash
   export OPENAI_API_KEY=your_openai_api_key
   ```

---

## 🎉 Getting Started

### The First Run Magic

On your first run, Zing will create a shiny new config file at `~/.config/zing/config.toml`. It's like your personalized spellbook for Zing!

```bash
zing
```

### Behold the Default Config

```toml
ai_provider = "openai" # Options: "openai", "ollama", "both"

[openai]
api_url = "https://api.openai.com/v1/completions"
model = "text-davinci-003"
token_limit = 500

[ollama]
api_url = "http://localhost:11434/generate"
model = "llama2"
token_limit = 500

[general]
exclude = []
max_file_size_kb = 100
use_both_models = false
verbose = false
confirm_commit = true
examples = [
  "feat: add user login functionality",
  "fix: resolve data parsing issue",
  "docs: update API documentation"
]
```

---

## 🎛 Configuration Galore

Make Zing truly yours by tweaking the `config.toml` file:

- **Choose Your AI Provider**: `openai`, `ollama`, or `both`—it's up to you!
- **Set Token Limits**: Control the verbosity (and cost) of AI-generated messages.
- **Customize Prompts**: Add your own example commit messages to guide the AI.
- **Exclude Files**: Keep certain files out of the commit with ease.
- **Max File Size**: Automatically skip or summarize large files.
- **Verbose Mode**: Need more details? Turn on verbose output.

---

## 🎯 Usage

Using Zing is as easy as 1-2-3!

### Commit Like a Pro

```bash
zing
```

Zing will:

1. Detect changed files.
2. Exclude files specified in your config.
3. Add changes to staging.
4. Generate an awesome commit message using AI.
5. Prompt you for confirmation.
6. Commit your changes with flair!

### Command-Line Options

- **Exclude Files on the Fly**

  ```bash
  zing -exclude="secrets.txt,debug.log"
  ```

- **Enable Verbose Output**

  ```bash
  zing -v
  ```

- **Skip Confirmation Prompt**

  ```bash
  zing -y
  ```

- **Need Help?**

  ```bash
  zing -h
  ```

---

## 🛡 Security First

Your code is precious, and we get that!

- **Environment Variables**: API keys are read from environment variables—no secrets in plain sight.
- **File Exclusions**: Easily exclude sensitive files from being sent to AI providers.
- **Privacy Matters**: Zing respects your data and only sends what's necessary.

---

## 🤖 Under the Hood

### AI Provider Flexibility

- **OpenAI**: For cutting-edge AI capabilities.
- **Ollama**: For local AI magic without external API calls.
- **Both**: Combine the strengths of both to optimize cost and performance.

### Smart AI Interactions

- **Contextual Prompts**: Zing crafts intelligent prompts with examples to get the best out of AI.
- **Token Management**: Keeps an eye on token usage to prevent unexpected costs.
- **Large Diff Handling**: Summarizes large changes to keep things efficient.

---

## 🌟 Real-World Examples

### The Speedy Commit

```bash
zing -y
```

No time to waste? Zing commits your changes with an AI-generated message, no questions asked!

### The Perfectionist's Choice

```bash
zing -v
```

See detailed output, review the generated message, and proceed when you're satisfied.

### The Custom Exclusion

```bash
zing -exclude="config.yml"
```

Keep that sensitive config file out of the commit without breaking a sweat.

---

## 💡 Tips and Tricks

- **Fine-Tune AI Responses**: Adjust `token_limit` and `temperature` in the config to control the creativity of AI.
- **Stay Updated**: Keep your AI models up to date for the best performance.
- **Contribute Examples**: Add more examples to your config to guide the AI's style.

---

## 🧙‍♂️ FAQs

**Q:** *What if I don't have an OpenAI API key?*

**A:** No worries! Set `ai_provider` to `ollama` and use a local AI model.

**Q:** *Can I use Zing with Git GUIs?*

**A:** Zing is designed for the command line, but you can integrate it into your workflow scripts.

**Q:** *Is my code safe with Zing?*

**A:** Absolutely! Zing is designed with security in mind. You control what gets sent to AI providers.

---

## 🤝 Contributing

Join the Zing community and help us make this tool even more amazing!

1. **Fork the Repo**

   Click that magical "Fork" button.

2. **Create a Feature Branch**

   ```bash
   git checkout -b feature/your-awesome-feature
   ```

3. **Commit with Zing**

   Let Zing generate those commit messages!

4. **Push and Open a PR**

   Share your brilliance with the world.

---

## 📄 License

[MIT License](LICENSE)

---

## 🎉 Join the Zing Revolution!

Ready to supercharge your Git commits? Give Zing a whirl and add some zing to your workflow!

---

## 📬 Contact

Got questions, ideas, or just want to say hi? Open an issue on GitHub.

---

## 🌟 Acknowledgments

- **You**: For being awesome and choosing Zing!
- **OpenAI & Ollama**: For powering the brains behind Zing.
- **Community Contributors**: For making Zing better every day.

---

*Zing—because committing code should be as exciting as writing it!*

[![Star this repo](https://img.shields.io/github/stars/crazywolf132/zing?style=social)](https://github.com/crazywolf132/zing/stargazers)

---

**Happy Coding!**
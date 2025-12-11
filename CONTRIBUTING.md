# Contributing to MemCortex

Thank you for considering contributing to **MemCortex**! 
MemCortex is an open-source, lightweight long-term memory layer for LLMs, built with simplicity and extensibility in mind. Contributions of all forms are welcome: code, documentation, ideas, bug reports, or feature requests.

---

## 🧱 Project Philosophy
MemCortex aims to be:
- **Lightweight** – small, fast, minimal dependencies.
- **Transparent** – easy to understand, debug, and extend.
- **Composable** – works with any LLM or inference server.
- **Open** – built openly, in the spirit of community-driven AI infrastructure.

If your contribution aligns with these principles, you're in the right place.

---

## 🚀 Getting Started

### 1. Fork the Repository
Click the **Fork** button at the top of the repository.

### 2. Clone Your Fork
```bash
git clone https://github.com/sobowalebukola/memcortex.git
cd memcortex
```

### 3. Install Dependencies
```bash
go mod download
```

---

## 🛠️ Development Workflow

### Create a New Branch
Use a descriptive branch name:
```bash
git checkout -b feature/memory-summarisation
```

### Commit Messages
Follow the conventional commits pattern:
- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `refactor:` code improvements
- `test:` testing updates

Example:
```
feat: add embedding cache eviction policy
```

### Linting
```bash
golangci-lint run
```
Fix any issues before submitting.

---

## 🧪 Testing
Tests are currently not available in the repository.

If you add new features or bug fixes, you are encouraged (but not required) to include tests to help improve long-term reliability.

---


## 🐞 Reporting Issues
Before creating an issue, check if one already exists.

When filing a new issue, include:
- A clear description of the problem or request
- Steps to reproduce (if it's a bug)
- Logs or screenshots (if applicable)
- Expected vs actual behavior

---

## 🔧 Feature Requests
Feature requests are welcome! Please explain:
- **Why** the feature is needed
- **How** it fits the MemCortex philosophy
- Possible implementation ideas

---

## 🤝 Pull Requests
When you're ready:
1. Push your branch
2. Open a Pull Request
3. Fill out the PR template

We encourage small, focused PRs — easier to review and merge.

---

## 📬 Community & Communication
If you want to discuss anything before building, feel free to:
- Open a GitHub Discussion
- Comment on an issue
- Ping maintainers in an existing thread

You’re welcome to share design ideas, proposals, or drafts.

---

## 🙌 Thank You
Every contribution — large or small — helps make MemCortex a more powerful and useful tool for the AI community.

Thanks again for being part of the journey!


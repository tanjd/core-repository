# Core Repository

A modern monorepo built with Nx, supporting Python, Go, and Next.js projects. This repository uses DevContainers for a consistent development environment across all contributors.

## 🛠 Tech Stack

- **Build System**: [Nx](https://nx.dev/) (v20.4.2)
- **Package Manager**: [pnpm](https://pnpm.io/)
- **Languages & Frameworks**:
  - Python (with Poetry)
  - Go (v1.23)
  - Next.js
  - React
- **Development Environment**: DevContainers
- **CI/CD**: GitHub Actions

## 📁 Repository Structure

```
.
├── apps/                  # Application projects
│   ├── identity/         # Go service
│   └── my-python-project/# Python application
├── libs/                 # Shared libraries
│   └── my-python-library/# Python library
└── ...
```

## 🚀 Getting Started

### Prerequisites

1. [Docker](https://www.docker.com/get-started)
2. [VS Code](https://code.visualstudio.com/)
3. [VS Code Remote - Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

### Development Setup

1. Clone the repository:

   ```bash
   git clone <repository-url>
   cd core-repository
   ```

2. Open in VS Code:

   ```bash
   code .
   ```

3. When prompted, click "Reopen in Container" or:

   - Press `F1`
   - Type "Reopen in Container"
   - Press Enter

4. The DevContainer will automatically:
   - Install all required tools (Node.js, Python, Go, etc.)
   - Set up development environment
   - Install VS Code extensions
   - Run `make setup` to install dependencies

### Development Commands

```bash
# Install dependencies
make setup

# Run tests for affected projects
pnpm test

# Lint affected projects
pnpm lint

# Format code
pnpm format

# Upgrade Nx
make upgrade-nx
```

## 🧱 Project Management

### Adding New Projects

#### Python Project

```bash
pnpm nx g @nxlv/python:poetry-project my-new-python-project
```

#### Go Project

```bash
pnpm nx g @nx-go/nx-go:project my-new-go-project
```

#### Next.js Project

```bash
pnpm nx g @nx/next:app my-new-next-app
```

### Running Projects

Use Nx to run any target (build, test, lint, etc.) for a specific project:

```bash
pnpm nx <target> <project>
# Example: pnpm nx test my-python-project
```

## 🛠 DevContainer Features

The development container includes:

- 🐍 Python environment with Poetry and pyenv
- 🟦 Node.js with pnpm
- 🔷 Go 1.23
- 🐳 Docker-in-Docker support
- 🔧 Pre-configured VS Code extensions
- 🔍 Code formatting and linting tools
- 🔄 Live Share support
- 🐚 ZSH with helpful plugins

## 📝 VS Code Configuration

The DevContainer comes with pre-configured settings for:

- Python formatting (Ruff)
- JSON/JSONC formatting
- TOML formatting
- Editor rulers and code style settings
- Auto-formatting on save
- Import organization
- And more...

## 🤝 Contributing

1. Create a new branch for your feature
2. Make your changes
3. Run tests and linting
4. Submit a pull request

## 📚 Additional Resources

- [Nx Documentation](https://nx.dev/getting-started/intro)
- [DevContainers Documentation](https://code.visualstudio.com/docs/remote/containers)
- [Poetry Documentation](https://python-poetry.org/docs/)
- [Go Documentation](https://golang.org/doc/)

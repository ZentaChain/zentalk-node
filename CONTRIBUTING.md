# Contributing to Zentalk Node

Thank you for your interest in contributing to Zentalk Node! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Coding Standards](#coding-standards)
- [Pull Request Process](#pull-request-process)
- [Issue Reporting](#issue-reporting)
- [Testing](#testing)
- [Documentation](#documentation)

## Code of Conduct

### Our Standards

- Be respectful and inclusive
- Focus on constructive feedback
- Accept criticism gracefully
- Prioritize the community's best interests
- Show empathy towards other contributors

### Unacceptable Behavior

- Harassment, discrimination, or offensive comments
- Trolling, insulting, or derogatory remarks
- Publishing others' private information
- Any conduct that could reasonably be considered inappropriate

## Getting Started

### Prerequisites

- **Go**: Version 1.21 or higher
- **SQLite3**: For database operations
- **Git**: For version control
- **GitHub Account**: To submit contributions

### Fork and Clone

```bash
# Fork the repository on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/zentalk-node.git
cd zentalk-node

# Add upstream remote
git remote add upstream https://github.com/ZentaChain/zentalk-node.git
```

## Development Setup

### 1. Install Dependencies

```bash
# Install Go dependencies
go mod download

# Verify installation
go version
```

### 2. Build the Project

```bash
# Build relay server
go build -o relay cmd/relay/main.go

# Build mesh storage server
go build -o mesh-api cmd/mesh-api/main.go
```

### 3. Run Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

### 4. Run Locally

```bash
# Start relay server
./relay --port 9001

# Start mesh storage (in another terminal)
./mesh-api --port 8081
```

## How to Contribute

### Types of Contributions

We welcome the following types of contributions:

- **Bug Fixes**: Fix issues and bugs in the codebase
- **New Features**: Implement new functionality
- **Documentation**: Improve or add documentation
- **Tests**: Add or improve test coverage
- **Performance**: Optimize existing code
- **Security**: Identify and fix security vulnerabilities

### Contribution Workflow

1. **Create an Issue**: Before starting work, create or comment on an issue
2. **Discuss**: Discuss your approach with maintainers
3. **Fork & Branch**: Create a feature branch from `main`
4. **Implement**: Write your code following our standards
5. **Test**: Ensure all tests pass and add new tests
6. **Commit**: Write clear, descriptive commit messages
7. **Push**: Push your changes to your fork
8. **Pull Request**: Submit a PR to the main repository

## Coding Standards

### Go Style Guide

Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) and [Effective Go](https://golang.org/doc/effective_go.html).

### Code Formatting

```bash
# Format all Go files
go fmt ./...

# Run linter (install golangci-lint first)
golangci-lint run
```

### Naming Conventions

- **Packages**: Short, lowercase, single-word names
- **Functions**: Use camelCase for private, PascalCase for exported
- **Variables**: Descriptive names, avoid single letters except in short loops
- **Constants**: Use PascalCase or UPPER_CASE for exported constants

### Error Handling

- Always check and handle errors
- Return errors instead of panicking (except in truly exceptional cases)
- Wrap errors with context using `fmt.Errorf("context: %w", err)`

### Example

```go
// Good
func ProcessMessage(msg *Message) error {
    if msg == nil {
        return fmt.Errorf("message cannot be nil")
    }

    if err := msg.Validate(); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    return nil
}

// Bad
func ProcessMessage(msg *Message) {
    msg.Validate() // Error not checked
    // ... rest of code
}
```

### Comments

- All exported functions, types, and constants must have comments
- Comments should explain **why**, not **what**
- Use complete sentences with proper punctuation

```go
// EncryptMessage applies multi-layer onion routing encryption to ensure
// sender anonymity and message confidentiality during relay transmission.
func EncryptMessage(plaintext []byte, relays []Relay) ([]byte, error) {
    // Implementation
}
```

## Pull Request Process

### Before Submitting

- [ ] Code follows the style guidelines
- [ ] All tests pass (`go test ./...`)
- [ ] New tests added for new features
- [ ] Documentation updated if necessary
- [ ] Commit messages are clear and descriptive
- [ ] No merge conflicts with `main` branch

### PR Guidelines

1. **Title**: Use a clear, descriptive title
   - Good: "Add DHT peer discovery timeout configuration"
   - Bad: "Fix bug" or "Update code"

2. **Description**: Include:
   - What changes were made
   - Why the changes were necessary
   - How to test the changes
   - Related issue number (if applicable)

3. **Commits**:
   - Keep commits focused and atomic
   - Write meaningful commit messages
   - Use present tense ("Add feature" not "Added feature")

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Related Issue
Fixes #(issue number)

## Testing
- How were these changes tested?
- What test cases were added?

## Checklist
- [ ] Code follows style guidelines
- [ ] Tests pass locally
- [ ] Documentation updated
- [ ] No breaking changes (or documented if necessary)
```

### Review Process

- Maintainers will review your PR within 3-5 business days
- Address review comments promptly
- Be open to feedback and suggestions
- PRs may require multiple rounds of review

## Issue Reporting

### Before Creating an Issue

1. **Search**: Check if the issue already exists
2. **Verify**: Ensure it's reproducible
3. **Update**: Make sure you're using the latest version

### Issue Template

```markdown
## Description
Clear description of the issue

## Steps to Reproduce
1. Step one
2. Step two
3. Step three

## Expected Behavior
What should happen

## Actual Behavior
What actually happens

## Environment
- OS: [e.g., Ubuntu 22.04]
- Go Version: [e.g., 1.21.5]
- Zentalk Node Version: [e.g., v0.1.0]

## Additional Context
Any other relevant information
```

## Testing

### Writing Tests

- Write tests for all new features
- Maintain or improve code coverage
- Use table-driven tests where appropriate
- Mock external dependencies

### Example Test

```go
func TestProcessMessage(t *testing.T) {
    tests := []struct {
        name    string
        msg     *Message
        wantErr bool
    }{
        {
            name:    "valid message",
            msg:     &Message{Content: "test"},
            wantErr: false,
        },
        {
            name:    "nil message",
            msg:     nil,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ProcessMessage(tt.msg)
            if (err != nil) != tt.wantErr {
                t.Errorf("ProcessMessage() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./pkg/protocol

# Run with race detection
go test -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Documentation

### Code Documentation

- Document all exported functions, types, and packages
- Use godoc format for comments
- Include usage examples in documentation

### README Updates

- Update README.md if adding new features
- Include configuration examples
- Add troubleshooting steps for common issues

### External Documentation

- Update protocol documentation in `zentalk-protocol` repo if changes affect protocol
- Document breaking changes clearly
- Provide migration guides when necessary

## Questions?

If you have questions:

- Open a [GitHub Discussion](https://github.com/ZentaChain/zentalk-node/discussions)
- Join our community (check README for links)
- Ask in your pull request or issue

## License

By contributing to Zentalk Node, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Zentalk Node and helping build a more private, decentralized internet!

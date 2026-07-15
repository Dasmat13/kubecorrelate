# Contributing to KubeCorrelate

Thank you for your interest in improving **KubeCorrelate**! We welcome contributions of all forms, including bug fixes, new features, documentation, and issues.

---

## 🛠️ Setting Up Your Local Environment

1. **Prerequisites:**
   * Go 1.24 or later
   * `kubectl` configured with a target Kubernetes cluster (or a local `minikube` / `kind` environment)

2. **Clone and Build:**
   ```bash
   git clone https://github.com/Dasmat13/kubecorrelate.git
   cd kubecorrelate
   go build -o bin/kubecorrelate cmd/kubecorrelate/main.go
   ```

---

## 🧪 Running Tests

Ensure all tests pass before submitting a pull request:
```bash
go test -v ./...
```

To add tests:
* Unit tests for watchers belong under `pkg/watcher/*_test.go`.
* Printer and console output tests belong under `pkg/printer/*_test.go`.

---

## 📝 Pull Request Guidelines

1. **Branch Naming:**
   Use descriptive branch names:
   * `feat/your-feature-name`
   * `fix/bug-description`
   * `chore/doc-or-build-updates`

2. **Commit Messages:**
   Follow Conventional Commit style for clean change logs:
   * `feat: add regex filtering to log output stream`
   * `fix: prevent potential race condition in printer slop buffer`
   * `chore: update setup instructions in README`

3. **Submitting changes:**
   * Ensure your code is formatted with `go fmt ./...`.
   * Push your branch to your fork and open a Pull Request against `main`.
   * Make sure the GitHub Actions build passes successfully.

# 👻 Ghostrun

**Trace Go code execution without running it.**

Ghostrun is a static code tracer for Go. Point it at any function, give it sample inputs, and watch it step through the code line-by-line — tracking variable values, following function calls across files, and resolving interface dispatch — all without compiling or executing anything.

Think of it as your pen-and-paper dry run, automated.

<p align="center">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go" />
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey?style=flat-square" />
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" />
</p>

---

## Why?

You're staring at a 50-file Go service. A request comes in, hits a handler, calls three functions, goes through an interface, branches on a condition, and returns. You want to understand the flow.

Your options today:
- **Debugger** — requires a running environment, compiled binary, database connections, the whole stack
- **AI "explain this code"** — hallucinates, misses branches, can't track actual values
- **Pen and paper** — accurate but slow, error-prone, doesn't scale

Ghostrun gives you a fourth option: **static interpretation**. It reads your Go source code, simulates execution using your sample inputs, and shows you exactly what happens — variable by variable, line by line, call by call.

---

## Quick Start

### Install

```bash
# Clone and build
git clone https://github.com/YOUR_USERNAME/ghostrun.git
cd ghostrun
go build -o ghostrun .

# Or install directly
go install github.com/YOUR_USERNAME/ghostrun@latest
```

### Trace a function

```bash
ghostrun trace ./path/to/package FunctionName \
  --input '{"param1": "value1", "param2": 42}'
```

This will:
1. Parse and type-check your Go code
2. Find the function
3. Simulate execution with your inputs
4. Open an interactive web UI at `http://localhost:8080`

### Browse a call graph

```bash
ghostrun navigate ./path/to/package
```

No inputs needed — just browse which functions call which.

---

## Example

Given this code:

```go
// order/order.go
func ProcessOrder(o Order) (OrderResult, error) {
    total := calculateTotal(o.Items)

    var disc Discounter
    if total > 200 {
        disc = PercentDiscount{Percent: 10}
    } else if total > 100 {
        disc = FlatDiscount{Amount: 20}
    }

    discount := applyDiscount(total, disc)
    final := total - discount

    return OrderResult{
        OrderID:  o.ID,
        Total:    total,
        Discount: discount,
        Final:    final,
    }, nil
}
```

Run:

```bash
ghostrun trace ./order ProcessOrder \
  --input '{"o": {"ID": 1, "Amount": 150, "Items": [{"Name": "Pizza", "Price": 100}, {"Name": "Coke", "Price": 50}]}}'
```

Ghostrun traces through:

```
ProcessOrder(o: Order{ID: 1, Amount: 150, Items: [...]})
  │
  ├─ total := calculateTotal(items)
  │   ├─ loop iteration 0: total = 0 + 100 = 100
  │   ├─ loop iteration 1: total = 100 + 50 = 150
  │   └─ return 150
  │
  ├─ if total > 200 → false, skipping
  ├─ if total > 100 → true, entering
  │   └─ disc = FlatDiscount{Amount: 20}
  │
  ├─ discount := applyDiscount(150, disc)
  │   ├─ if d == nil → false
  │   ├─ d.Calculate(150)  ← interface dispatch → FlatDiscount.Calculate
  │   │   └─ return 20
  │   └─ return 20
  │
  ├─ final = 150 - 20 = 130
  │
  └─ return (OrderResult{OrderID: 1, Total: 150, Discount: 20, Final: 130}, nil)
```

All of this is displayed in an interactive three-panel web UI:

| Panel | What it shows |
|-------|---------------|
| **Call Tree** (left) | Expandable tree of function calls. Click any node to jump to it. |
| **Source Code** (center) | Highlighted source with the current line marked. |
| **Variables** (right) | All variable values at the selected step. |

---

## Features

### 🔍 Two Modes

| Mode | Command | What it does |
|------|---------|-------------|
| **Trace** | `ghostrun trace <pkg> <func> --input '{...}'` | Full step-by-step execution with variable tracking |
| **Navigate** | `ghostrun navigate <pkg>` | Static call graph — browse which functions call which |

### 🧠 What Ghostrun Understands

| Go Feature | Supported |
|------------|-----------|
| Functions & methods | ✅ |
| Structs & field access | ✅ |
| Interfaces & method dispatch | ✅ Resolves to concrete type |
| Slices & maps | ✅ |
| Pointers | ✅ |
| Multi-return values | ✅ |
| for loops | ✅ |
| for range (slices, maps) | ✅ |
| if/else if/else | ✅ |
| switch/case | ✅ |
| Short variable declarations (`:=`) | ✅ |
| Compound assignments (`+=`, `-=`) | ✅ |
| break, continue | ✅ |
| Builtins (len, append, make) | ✅ |
| Goroutines | ❌ (to be scoped) |
| Channels | ❌ (planned) |
| Defer | ❌ (planned) |
| Closures | ❌ (planned) |

### ⌨️ Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `→` or `l` | Next step |
| `←` or `h` | Previous step |
| `↓` or `j` | Next step |
| `↑` or `k` | Previous step |
| `Home` or `r` | Restart (go to first step) |
| `End` | Go to last step |

### 🔌 Stub System

External calls (database, HTTP, etc.) can't be simulated. Register stubs to provide return values:

```go
interp := interpreter.New(pd)

// Stub a database call
interp.RegisterStub("db.FindUser", func(args []value.Value) value.Value {
    return value.NewStruct("User", map[string]value.Value{
        "ID":   value.NewInt(1),
        "Name": value.NewString("Vedant"),
    })
})
```

---

## Architecture

```
ghostrun trace ./pkg ProcessOrder --input '{...}'
         │
         ▼
┌─────────────────────┐
│   Package Loader    │  go/packages + go/types
│   Type Checker      │  Full type resolution
│   Function Registry │  All functions indexed
└────────┬────────────┘
         │
┌────────▼────────────┐
│   AST Interpreter   │
│   ├─ Scope Chain    │  Variable bindings per scope
│   ├─ Expr Evaluator │  Evaluate Go expressions
│   ├─ Stmt Executor  │  Execute Go statements
│   ├─ Call Resolver  │  Functions + interface dispatch
│   └─ Trace Recorder │  Record every step
└────────┬────────────┘
         │
┌────────▼────────────┐
│   Web Server        │
│   ├─ /api/trace     │  Trace tree JSON
│   ├─ /api/callgraph │  Call graph JSON
│   ├─ /api/source    │  Source code
│   └─ Embedded UI    │  Three-panel viewer
└─────────────────────┘
```

### How it works (not a debugger)

| | Debugger (dlv) | Ghostrun |
|---|---|---|
| Runs real code? | ✅ Compiled binary | ❌ AST interpretation |
| Needs build? | ✅ Must compile | ❌ Just parses source |
| Side effects? | ⚠️ DB writes, HTTP, etc. | ❌ None |
| External deps? | ✅ Real DB, APIs | 🔶 You provide stubs |
| Goroutines? | ✅ Full support | ❌ Single-threaded only |
| Setup needed? | Environment, configs, data | Just the source code |

---

## Project Structure

```
ghostrun/
├── main.go                        # CLI entry point
├── cmd/
│   ├── root.go                    # Root command
│   ├── trace.go                   # ghostrun trace
│   └── navigate.go                # ghostrun navigate
├── internal/
│   ├── value/                     # Value type system
│   ├── scope/                     # Scope chain
│   ├── trace/                     # Trace recorder
│   ├── loader/                    # Package loader (go/packages)
│   ├── interpreter/               # AST tree-walking interpreter
│   │   ├── interpreter.go         # Orchestrator
│   │   ├── expr.go                # Expression evaluation
│   │   ├── stmt.go                # Statement execution
│   │   └── call.go                # Call resolution
│   ├── callgraph/                 # Static call graph builder
│   └── server/                    # HTTP server + embedded UI
│       ├── server.go
│       └── ui/                    # Web frontend
│           ├── index.html
│           ├── app.js
│           └── style.css
└── testdata/basic/                # Sample Go project for testing
```

---

## CLI Reference

### `ghostrun trace`

```
Usage:
  ghostrun trace [package-path] [function-name] [flags]

Flags:
  -i, --input string   JSON input for function parameters
  -p, --port int       Port for the web UI (default 8080)
  -h, --help           help for trace
```

**Input format**: JSON object where keys match the function's parameter names.

```bash
# Function: func Process(id int, name string) error
ghostrun trace ./pkg Process --input '{"id": 42, "name": "test"}'

# Function: func Handle(req Request) Response
ghostrun trace ./pkg Handle --input '{"req": {"Method": "GET", "Path": "/api"}}'
```

### `ghostrun navigate`

```
Usage:
  ghostrun navigate [package-path] [flags]

Flags:
  -p, --port int   Port for the web UI (default 8080)
  -h, --help       help for navigate
```

---

## Contributing

PRs welcome! The architecture is modular — each package in `internal/` is independent and testable:

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/interpreter/ -v

# Build
go build -o ghostrun .
```

---

## License

MIT

---

<p align="center">
  <strong>👻 Ghostrun</strong> — because the best way to understand code is to walk through it.
</p>

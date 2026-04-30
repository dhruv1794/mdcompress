# full-feature corpus

> A blockquote at the top of the file. Some projects open with these.

## Install

```bash
go install github.com/example/example@latest
```

Or via Homebrew:

```sh
brew install example
```

## Usage

The CLI accepts both stdin and a path argument:

| Form          | Behavior                       |
|---------------|--------------------------------|
| `example`     | reads stdin, writes stdout     |
| `example x.md`| reads `x.md`, writes stdout    |

1. First, install the binary.
2. Then, point it at a file.
3. Pipe the result anywhere.

## API

```go
package main

import "github.com/example/example/pkg/lib"

func main() {
    lib.Do()
}
```

A paragraph with *emphasis*, **strong**, and `inline code`. Visit
[the homepage](https://example.com) for more details.

---

# Setext Heading
## Another setext heading
- nested
  - list
    - depth
- back to top

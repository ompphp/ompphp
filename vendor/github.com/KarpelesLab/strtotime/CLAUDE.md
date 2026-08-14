# CLAUDE.md - Guidelines for strtotime Go library

## Build Commands
- `make` - Format code with goimports and build project
- `make deps` - Get project dependencies
- `make test` - Run all tests
- `go test -v -run=TestName` - Run a specific test

## Linting/Formatting
- Use goimports for formatting (`make` runs it automatically)
- Run `go vet` to check for common errors
- Run `golint` for style issues

## Code Style Guidelines
- Follow standard Go conventions: camelCase for variables, PascalCase for exports
- Use explicit error handling with returns, not panics
- Errors should be descriptive and use proper error wrapping
- Group imports: standard lib first, then third-party
- Keep functions small and focused on single responsibility
- Use descriptive variable names (especially for non-trivial types)
- Include comments for exported functions (godoc style)
- Error messages should be lowercase and without punctuation

## Project Structure
This library parses time strings into Go time.Time objects with support for:
- Natural language time parsing
- Custom parsers for different languages
- Partial string parsing
- Extensible architecture via options
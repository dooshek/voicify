# Coding Standards and Practices

## Package Usage
- Always use `logger` package for logging, do not use `fmt` or `print` functions except for using `logger` file: internal/config/wizard.go (reason: interactive CLI wizard)
- Always use `internal/notification/notification.go` package for sending desktop notifications
- Always use `config` package for reading and writing configuration
- Always use `keyboard` package for handling global keyboard shortcuts
- Always use `audio` package for recording audio
- Always use `transcriber` package for transcribing audio
- Always use `fileops` package for file operations
- Always use `clipboard` package for clipboard operations

## CLI Arguments
- Arguments to command line interface should always include `--` prefix (not `-`)

## String Escaping
- In Go strings, backslash (\) is an escape character
- To display a literal backslash, use double backslash (\\)
- Example:
  - WRONG: `Only a-z, 0-9, and `[]\;',./-= keys are allowed`
  - CORRECT: `Only a-z, 0-9, and `[]\\;',./-= keys are allowed`

## Common Mistakes to Avoid
- Using single backslash before semicolon (\;) - this creates an invalid escape sequence
- Always use double backslash (\\) when you want to show a literal backslash in strings

## Logging Patterns

1. Use `logger.Errorf` with the error as the second parameter:
   ```go
   // Correct pattern
   logger.Errorf("Message with format %s", err, formatArg)

   // Incorrect pattern
   logger.Errorf("Message with format %s", formatArg, err)
   ```

2. Use `logger.Error` when no additional arguments are required:
   ```go
   // When only displaying the error
   logger.Error("Failed operation", err)
   ```

3. The logger functions follow this pattern: first argument is the message, second is the error (if applicable), and subsequent arguments are for sprintf formatting.

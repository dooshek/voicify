# Commit Management

## Conventional Commits

When the user writes "commit" (without quotes), automatically:

1. Identify all changed files that need to be committed
2. Format a proper commit message following the [Conventional Commits specification](https://www.conventionalcommits.org/en/v1.0.0/)
3. The commit message should follow this format:
   ```<type>[optional scope]: <description>```

4. Common types include:
   - `feat`: A new feature
   - `fix`: A bug fix
   - `docs`: Documentation only changes
   - `style`: Changes that do not affect the meaning of the code
   - `refactor`: A code change that neither fixes a bug nor adds a feature
   - `perf`: A code change that improves performance
   - `test`: Adding missing tests or correcting existing tests
   - `build`: Changes that affect the build system or external dependencies
   - `ci`: Changes to CI configuration files and scripts
   - `chore`: Other changes that don't modify src or test files

5. The description should:
   - Use imperative, present tense: "change" not "changed" nor "changes"
   - Not capitalize the first letter
   - Not end with a period

6. Add relevant scope based on the changes (e.g., `feat(plugin)`, `fix(audio)`, etc.)

7. Always create a meaningful commit message that accurately describes what changed

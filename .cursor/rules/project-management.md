# Project Management

- If you add packages to the project, add them to the go.mod file
- When adding, removing, or modifying functionality, update the README.md file accordingly:
  - Update Usage section if command line arguments change
  - Update Features section if functionality changes
  - Update Requirements if dependencies change
  - Update Components section if packages are added/removed
  - Update Configuration section if config structure changes

## Dependencies Management
- Do not add new libraries or packages unless absolutely necessary
- Always ask user's permission before suggesting new dependencies
- Try to use standard library solutions when possible
- When suggesting a new dependency, explain why it's necessary and what alternatives were considered

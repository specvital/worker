# specvital worker

Background job processing service for SpecVital platform.

## Architecture

Clean Architecture with separate entry points for Railway deployment:

```
src/cmd/
├── analyzer/       # Analysis worker - parse test files (Railway service)
├── spec-generator/ # SpecView worker - AI-powered spec generation
├── enqueue/        # CLI tool for manual task enqueue
```

## Build

```bash
# Build all binaries
just build

# Build specific target
just build analyzer
just build spec-generator
just build enqueue

# Output: bin/analyzer, bin/spec-generator, bin/enqueue
```

## Development

```bash
# Run analyzer locally with hot reload
just run-analyzer local

# Run spec-generator locally
just run-spec-generator local

# Run tests
just test unit
just test integration
just test all
```

## Environment Variables

- `DATABASE_URL`: PostgreSQL connection string (also used for river job queue)

## Railway Deployment

Deploy worker services (horizontally scalable):

- **Analyzer**: `bin/analyzer` - processes analysis tasks from queue
- **Spec Generator**: `bin/spec-generator` - generates AI-powered spec documents

## License

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

### Trademark Notice

"SpecVital" and the SpecVital logo are trademarks of KubrickCode. Forks and derivative works must use a different name and branding. See the [NOTICE](NOTICE) file for details.

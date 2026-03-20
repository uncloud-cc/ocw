# OCW Examples

This folder contains practical examples demonstrating various OCW workflow features.

## Quick Start

```bash
# Run any example
ocw 01-hello-world.yaml

# Run with custom env file
ocw -e production.env 05-environment-variables.yaml

# Run with secrets visible (for debugging)
ocw --show-secrets 08-secrets.yaml
```

## Examples by Category

### Basics
1. **01-hello-world.yaml** - Simple sequential workflow
2. **02-build-container.yaml** - Building a container image
3. **03-build-and-run.yaml** - Building then running a container

### Flow Control
4. **04-parallel-steps.yaml** - Running steps in parallel
5. **05-nested-flow.yaml** - Nesting parallel and sequence
6. **06-conditionals.yaml** - Switch/case conditional logic

### Templating & Data
7. **07-templating.yaml** - Using template expressions
8. **08-secrets.yaml** - Working with environment variables and secrets
9. **09-step-outputs.yaml** - Passing data between steps

### Development & Deployment
10. **10-jobs.yaml** - Multiple entry points in one file
11. **11-expose-service.yaml** - Exposing containers for local development
12. **12-container-networking.yaml** - Background containers and networking

### Real-World Patterns
13. **13-ci-pipeline.yaml** - Complete CI/CD pipeline example
14. **14-local-dev.yaml** - Full development environment with:
    - Node.js + Express server with hot reload (nodemon)
    - PostgreSQL database with migrations
    - Redis cache
    - API key authentication
    - Multiple jobs: dev, logs, db-shell, db-migrate, redis-cli, test, stop
    - Real podman logs command implementation

## Supporting Files

- `.env` - Environment variables and secrets (copy from .env.example)
- `.env.example` - Template showing required variables
- `Dockerfile` - Sample container for build examples
- `index.html` - Sample web page
- `nodejs/` - Complete Node.js application with hot reload
  - `server.js` - Express server with DB/Redis connections
  - `package.json` - Dependencies (express, pg, redis, nodemon)
  - `Dockerfile` - Dev container with hot reload support
  - `README.md` - Node.js app documentation

## Running the Full Development Example

The `14-local-dev.yaml` is a comprehensive, production-ready development environment:

```bash
# Start the full development environment (database + cache + app)
ocw dev 14-local-dev.yaml

# View application logs in real-time
ocw logs 14-local-dev.yaml

# Connect to database with psql
ocw dbshell 14-local-dev.yaml

# Run database migrations
ocw dbmigrate 14-local-dev.yaml

# Open Redis CLI
ocw rediscli 14-local-dev.yaml

# Test all services
ocw test 14-local-dev.yaml

# Stop all services
ocw stop 14-local-dev.yaml
```

### Secrets Configuration

The 14-local-dev.yaml uses these secrets (set them in your `.env` file):

```bash
DB_USER=dev
DB_PASSWORD=your_secure_password_here
API_KEY=your_api_key_here
```

All secrets are masked in output as `[secret]` unless you use `--show-secrets` flag.

### Hot Reload

The Node.js server uses nodemon for automatic reloading when you edit `nodejs/server.js`. No need to restart the container!

### API Endpoints

Once running, the application exposes:
- http://localhost:3000/ - Welcome message
- http://localhost:3000/health - Health check with DB/Redis status
- http://localhost:3000/api/data - Protected endpoint (requires X-API-Key header)

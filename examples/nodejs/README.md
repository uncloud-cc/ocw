# Node.js Fullstack Example

A complete fullstack example demonstrating how to use OCW workflows with jobs for local development.

## Stack

- **Frontend**: React + Vite dev server (port 5173)
- **Backend**: Express.js REST API (port 3001)

## Project Structure

```
nodejs/
├── workflows.yaml      # OCW workflow with jobs
├── backend/
│   ├── Dockerfile
│   ├── package.json
│   └── src/
│       ├── index.js    # Express API server
│       └── migrate.js  # Database migrations
└── frontend/
    ├── Dockerfile
    ├── package.json
    ├── vite.config.js
    ├── index.html
    └── src/
        ├── main.jsx
        ├── App.jsx
        └── index.css
```

## Available Jobs

```bash
# List available jobs
cd examples/nodejs
ocw

# Build both images
ocw build

# Start full development environment
ocw dev

# Run tests
ocw test
```

## Jobs Description

### `build`
Builds both API and frontend Docker images in parallel.

### `dev`
Full local development environment:
1. Builds API and frontend images
2. Installs dependencies for both
3. Starts API server and Vite dev server in parallel

### `test`
Runs the test suite (placeholder for now).

## Endpoints

Once running with `ocw dev`:

- **Frontend**: http://localhost:5173
- **API**: http://localhost:3001
- **API Health Check**: http://localhost:3001/health

### API Routes

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check with database status |
| GET | `/api/todos` | List all todos |
| POST | `/api/todos` | Create a new todo |
| PATCH | `/api/todos/:id` | Toggle todo completion |
| DELETE | `/api/todos/:id` | Delete a todo |

## Development

The workflow mounts the directory containing `workflows.yaml` as `/workflow` in all containers. This means:

- `/workflow/backend` → `./backend`
- `/workflow/frontend` → `./frontend`

Changes to source files will trigger hot reload when using the dev servers.

### Environment Variables

**Backend:**
- `PORT` - API server port (default: 3001)
- `DATABASE_URL` - PostgreSQL connection string
- `NODE_ENV` - Environment mode

**Frontend:**
- `VITE_API_URL` - Backend API URL for proxy configuration

## How It Works

The `workflows.yaml` uses OCW jobs to define named entry points:

```yaml
jobs:
  build:
    parallel:
      - build: { image: api:dev, context: /workflow/backend }
      - build: { image: frontend:dev, context: /workflow/frontend }

  dev:
    sequence:
      - # Build images
      - # Install dependencies  
      - # Start servers in parallel
```

This provides a docker-compose-like experience with the flexibility of named jobs for different workflows (build, dev, test, etc).

# Node.js Development Application

This is a fully functional Express.js application for the OCW local development example.

## Features

- **Express.js** web framework
- **Hot reload** with nodemon
- **PostgreSQL** database connectivity
- **Redis** caching layer
- **API key authentication** for protected endpoints
- **Health check** endpoint
- **Graceful shutdown** handling

## Files

- `server.js` - Main application server
- `package.json` - Node.js dependencies
- `Dockerfile` - Container definition with nodemon for hot reload

## API Endpoints

- `GET /` - Welcome message and endpoint list
- `GET /health` - Health check with database/cache status
- `GET /api/data` - Protected data endpoint (requires X-API-Key header)

## Usage

This is used by the `14-local-dev.yaml` workflow. Start the full development environment:

```bash
cd /Users/jonas/dev/ocw/examples
ocw dev 14-local-dev.yaml
```

Then visit:
- http://localhost:3000 - Application
- http://localhost:3000/health - Health check

The server will automatically reload when you make changes to `server.js`.

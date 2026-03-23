# Express TypeScript Server

A complete Express.js TypeScript server with Docker support and watch mode for development.

## Project Structure

```
nodejs/
├── src/
│   └── index.ts          # TypeScript source code
├── dist/                 # Compiled JavaScript (generated)
├── Dockerfile            # Multi-stage build for production
├── package.json          # Dependencies and scripts
├── tsconfig.json         # TypeScript configuration
└── pnpm-lock.yaml        # Locked dependencies
```

## Development

### Install Dependencies

```bash
cd nodejs
pnpm install
```

### Run TypeScript Compiler

```bash
pnpm build
```

### Start the Server

```bash
pnpm start
```

### Watch Mode (in OCW)

The server can be run with automatic rebuild and reload when source files change:

```bash
cd ..
ocw watch.yaml
```

This will:
1. Build the Docker image with the current source code
2. Start the container in the background
3. Watch for changes to `src/**/*.ts` files
4. Automatically rebuild the image and restart the container when changes are detected

## API Endpoints

- `GET /` - Returns server info
- `GET /health` - Health check endpoint
- `GET /api/info` - Server status and uptime

## Features

- **TypeScript**: Full type safety
- **Express 5**: Latest version with modern features
- **Docker**: Multi-stage build for optimized production images
- **Watch Mode**: Automatic rebuild and reload on file changes
- **Health Checks**: Built-in health check endpoint

## Environment Variables

- `PORT` - Server port (default: 3000)
- `NODE_ENV` - Environment name (default: development)

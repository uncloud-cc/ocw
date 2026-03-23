# Build stage
FROM node:25-alpine AS builder

WORKDIR /app

# Copy package files from build context
COPY package.json pnpm-lock.yaml ./

# Install pnpm
# Source: https://pnpm.io/installation#using-other-package-managers
RUN npm install -g pnpm@latest-10

# Install all dependencies (including dev for build)
RUN pnpm install --frozen-lockfile

# Copy source code
COPY tsconfig.json ./
COPY src ./src

# Build TypeScript
RUN pnpm build

# Runtime stage
FROM node:25-alpine

WORKDIR /app

# Copy dependencies from builder
COPY --from=builder /app/node_modules ./node_modules

# Copy compiled dist from builder
COPY --from=builder /app/dist ./dist

EXPOSE 3000

CMD ["node", "dist/index.js"]

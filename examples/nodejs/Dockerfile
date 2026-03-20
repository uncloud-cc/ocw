# Development Dockerfile for Node.js app with hot reload
FROM node:20-alpine

# Install nodemon globally for hot reloading
RUN npm install -g nodemon

# Create app directory
WORKDIR /app

# Copy package files and install dependencies at build time
COPY package*.json ./
RUN npm install

# Don't copy server.js - it will be mounted from /workflow/nodejs at runtime
# This enables hot reload to work properly

# Expose port
EXPOSE 3000

# Use nodemon for development with hot reload
# Files will be mounted from /workflow/nodejs
CMD ["npm", "run", "dev"]

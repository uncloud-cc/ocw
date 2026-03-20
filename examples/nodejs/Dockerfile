# Development Dockerfile for Node.js app with hot reload
FROM node:20-alpine

# Install nodemon globally for hot reloading
RUN npm install -g nodemon

# Create app directory
WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm install

# Copy app source
COPY server.js ./

# Expose port
EXPOSE 3000

# Use nodemon for development with hot reload
CMD ["npm", "run", "dev"]

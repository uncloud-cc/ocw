FROM node:25-alpine

# Copy package files
COPY package.json tsconfig.json ./

# Install dependencies
RUN npm install

# Copy source code
COPY server.ts ./

# Build TypeScript
RUN npm run build

# Start the server
CMD ["npm", "start"]

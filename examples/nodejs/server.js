const express = require('express');
const { Pool } = require('pg');
const { createClient } = require('redis');

const app = express();
const port = process.env.APP_PORT || 3000;

// Database configuration
const dbConfig = {
  host: process.env.DB_HOST || 'db',
  port: process.env.DB_PORT || 5432,
  database: process.env.DB_NAME || 'devdb',
  user: process.env.DB_USER || 'dev',
  password: process.env.DB_PASSWORD || 'devpass'
};

// Redis configuration
const redisConfig = {
  host: process.env.REDIS_HOST || 'cache',
  port: process.env.REDIS_PORT || 6379
};

// Initialize connections
let pgPool = null;
let redisClient = null;

async function initConnections() {
  try {
    // PostgreSQL
    pgPool = new Pool(dbConfig);
    const dbResult = await pgPool.query('SELECT NOW()');
    console.log('✅ Connected to PostgreSQL');
    
    // Redis
    redisClient = createClient({ socket: redisConfig });
    await redisClient.connect();
    console.log('✅ Connected to Redis');
  } catch (err) {
    console.warn('⚠️  Database connections failed:', err.message);
    console.log('   App will run without database features');
  }
}

// Middleware
app.use(express.json());

// Health check endpoint
app.get('/health', async (req, res) => {
  const status = {
    status: 'healthy',
    timestamp: new Date().toISOString(),
    version: process.env.APP_VERSION || '1.0.0',
    database: pgPool ? 'connected' : 'disconnected',
    cache: redisClient ? 'connected' : 'disconnected'
  };
  res.json(status);
});

// API key protected endpoint
app.get('/api/data', async (req, res) => {
  const apiKey = req.headers['x-api-key'];
  const validApiKey = process.env.API_KEY;
  
  if (validApiKey && apiKey !== validApiKey) {
    return res.status(401).json({ error: 'Invalid API key' });
  }
  
  try {
    let data = null;
    
    // Try to get from Redis cache first
    if (redisClient) {
      const cached = await redisClient.get('app:data');
      if (cached) {
        return res.json({ source: 'cache', data: JSON.parse(cached) });
      }
    }
    
    // Get from database
    if (pgPool) {
      const result = await pgPool.query('SELECT NOW() as time');
      data = { time: result.rows[0].time, message: 'Hello from PostgreSQL!' };
      
      // Cache the result
      if (redisClient) {
        await redisClient.set('app:data', JSON.stringify(data), { EX: 60 });
      }
    } else {
      data = { message: 'Hello from OCW dev server!', note: 'No database connected' };
    }
    
    res.json({ source: 'database', data });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// Home endpoint
app.get('/', (req, res) => {
  res.json({
    message: 'Welcome to OCW Development Server!',
    endpoints: [
      'GET /health - Health check',
      'GET /api/data - Protected data endpoint (requires X-API-Key header)'
    ],
    features: {
      hotReload: true,
      database: !!pgPool,
      cache: !!redisClient
    }
  });
});

// Start server
app.listen(port, '0.0.0.0', () => {
  console.log('🚀 Development server running!');
  console.log(`   URL: http://localhost:${port}`);
  console.log(`   Health: http://localhost:${port}/health`);
  console.log('');
  console.log('Features:');
  console.log('  ✓ Hot-reload enabled (nodemon)');
  console.log(`  ✓ Database: ${dbConfig.host}:${dbConfig.port}`);
  console.log(`  ✓ Cache: ${redisConfig.host}:${redisConfig.port}`);
  console.log('');
  console.log('Press Ctrl+C to stop');
  
  initConnections();
});

// Graceful shutdown
process.on('SIGTERM', async () => {
  console.log('Shutting down gracefully...');
  if (pgPool) await pgPool.end();
  if (redisClient) await redisClient.disconnect();
  process.exit(0);
});

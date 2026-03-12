const http = require('http');
const url = require('url');

// In-memory data store for demo purposes
let items = [
  { id: 1, name: 'Item 1', description: 'First item' },
  { id: 2, name: 'Item 2', description: 'Second item' }
];
let nextId = 3;

// Helper function to parse JSON body
const parseBody = (req) => {
  return new Promise((resolve, reject) => {
    let body = '';
    req.on('data', chunk => {
      body += chunk.toString();
    });
    req.on('end', () => {
      try {
        resolve(body ? JSON.parse(body) : {});
      } catch (error) {
        reject(error);
      }
    });
  });
};

// Helper function to send JSON response
const sendJSON = (res, statusCode, data) => {
  res.writeHead(statusCode, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(data));
};

// Request handler
const requestHandler = async (req, res) => {
  const parsedUrl = url.parse(req.url, true);
  const path = parsedUrl.pathname;
  const method = req.method;

  console.log(`${method} ${path}`);

  try {
    // Root endpoint - Hello World
    if (path === '/' && method === 'GET') {
      sendJSON(res, 200, { 
        message: 'Hello World!', 
        version: '1.0.0',
        endpoints: [
          'GET /',
          'GET /api/items',
          'POST /api/items',
          'PUT /api/items/:id',
          'DELETE /api/items/:id'
        ]
      });
    }
    // GET all items
    else if (path === '/api/items' && method === 'GET') {
      sendJSON(res, 200, { success: true, data: items });
    }
    // POST new item
    else if (path === '/api/items' && method === 'POST') {
      const body = await parseBody(req);
      const newItem = {
        id: nextId++,
        name: body.name || 'Unnamed',
        description: body.description || ''
      };
      items.push(newItem);
      sendJSON(res, 201, { success: true, data: newItem });
    }
    // PUT update item
    else if (path.match(/^\/api\/items\/\d+$/) && method === 'PUT') {
      const id = parseInt(path.split('/')[3]);
      const body = await parseBody(req);
      const itemIndex = items.findIndex(item => item.id === id);
      
      if (itemIndex === -1) {
        sendJSON(res, 404, { success: false, error: 'Item not found' });
      } else {
        items[itemIndex] = {
          ...items[itemIndex],
          name: body.name || items[itemIndex].name,
          description: body.description || items[itemIndex].description
        };
        sendJSON(res, 200, { success: true, data: items[itemIndex] });
      }
    }
    // DELETE item
    else if (path.match(/^\/api\/items\/\d+$/) && method === 'DELETE') {
      const id = parseInt(path.split('/')[3]);
      const itemIndex = items.findIndex(item => item.id === id);
      
      if (itemIndex === -1) {
        sendJSON(res, 404, { success: false, error: 'Item not found' });
      } else {
        const deletedItem = items.splice(itemIndex, 1)[0];
        sendJSON(res, 200, { success: true, data: deletedItem });
      }
    }
    // 404 Not Found
    else {
      sendJSON(res, 404, { success: false, error: 'Not found' });
    }
  } catch (error) {
    console.error('Error:', error);
    sendJSON(res, 400, { success: false, error: 'Bad request' });
  }
};

// Create server
const PORT = process.env.PORT || 3000;
const server = http.createServer(requestHandler);

server.listen(PORT, () => {
  console.log(`Server running at http://localhost:${PORT}/`);
  console.log(`Try: curl http://localhost:${PORT}/`);
});

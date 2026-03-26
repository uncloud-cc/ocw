import express, { Request, Response } from 'express'

const app = express()
const port = process.env.PORT || 3000

app.use(express.json())

app.get('/', (req: Request, res: Response) => {
  res.json({
    message: 'Hello from Express TypeScript Server! 🎉',
    timestamp: new Date().toISOString(),
    version: '1.0.0'
  })
})

app.get('/health', (req: Request, res: Response) => {
  res.json({ status: 'healthy' })
})

app.get('/api/info', (req: Request, res: Response) => {
  res.json({
    server: 'Express TypeScript',
    environment: process.env.NODE_ENV || 'development',
    uptime: process.uptime()
  })
})

app.listen(port, () => {
  console.log(`Server listening on port ${port}`)
  console.log(`Try: http://localhost:${port}/`)
})

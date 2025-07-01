#!/usr/bin/env node

const http = require('http');

const PORT = process.env.MOCK_PORT || 8080;

// Mock responses for different endpoints
const mockResponses = {
    '/api/v1/agents': {
        status: 200,
        body: [
            { uuid: 'devops-123', name: 'DevOps Bot' },
            { uuid: 'security-456', name: 'security' },
            { uuid: 'code-review-789', name: 'code-review' },
            { uuid: 'docs-012', name: 'docs' }
        ]
    },
    '/api/v1/chat': {
        status: 200,
        stream: true,
        messages: [
            { type: 'chat', content: 'Hello! How can I assist you today?' },
            { type: 'system', content: 'Processing your request...' },
            { type: 'chat', content: 'I understand you need help with...' }
        ]
    }
};

const server = http.createServer((req, res) => {
    console.log(`${req.method} ${req.url}`);

    // Add CORS headers
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');

    // Handle OPTIONS requests
    if (req.method === 'OPTIONS') {
        res.writeHead(204);
        res.end();
        return;
    }

    // Check API key
    const apiKey = req.headers['authorization'];
    if (!apiKey || apiKey !== 'Bearer test-api-key') {
        res.writeHead(401);
        res.end(JSON.stringify({ error: 'Invalid API key' }));
        return;
    }

    const mockResponse = mockResponses[req.url];
    if (!mockResponse) {
        res.writeHead(404);
        res.end(JSON.stringify({ error: 'Not found' }));
        return;
    }

    res.writeHead(mockResponse.status, { 'Content-Type': 'application/json' });
    
    if (mockResponse.stream) {
        // Simulate streaming response
        mockResponse.messages.forEach((msg, index) => {
            setTimeout(() => {
                res.write(JSON.stringify(msg) + '\n');
                if (index === mockResponse.messages.length - 1) {
                    res.end();
                }
            }, index * 100);
        });
    } else {
        res.end(JSON.stringify(mockResponse.body));
    }
});

server.listen(PORT, () => {
    console.log(`Mock server running on port ${PORT}`);
});

// Handle shutdown
process.on('SIGTERM', () => {
    server.close(() => {
        console.log('Mock server shutting down');
        process.exit(0);
    });
}); 
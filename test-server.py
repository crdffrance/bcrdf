#!/usr/bin/env python3
import http.server
import socketserver
import os
import json

PORT = 8080

# Change to the dist directory to serve the new binaries
os.chdir('dist')

class MyHTTPRequestHandler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header('Access-Control-Allow-Origin', '*')
        super().end_headers()

print(f"🚀 Test server starting on http://localhost:{PORT}")
print(f"📁 Serving files from: {os.getcwd()}")
print("📋 Available files:")
for file in os.listdir('.'):
    if file.startswith('bcrdf-'):
        print(f"   - {file}")
print("✅ Server ready! Press Ctrl+C to stop")

with socketserver.TCPServer(("", PORT), MyHTTPRequestHandler) as httpd:
    httpd.serve_forever()

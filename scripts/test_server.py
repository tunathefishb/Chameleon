from http.server import HTTPServer, BaseHTTPRequestHandler

class MyHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/':
            self.send_response(200)
            self.send_header('Content-Type', 'text/html')
            self.end_headers()
            self.wfile.write(b'<html><body><img src="//127.0.0.1:8000/image.jpg"></body></html>')
        elif self.path == '/image.jpg':
            self.send_response(200)
            self.send_header('Content-Type', 'image/jpeg')
            self.end_headers()
            self.wfile.write(b'fake_image_data')

server = HTTPServer(('127.0.0.1', 8000), MyHandler)
server.serve_forever()

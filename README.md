# Vector Search API

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/doc/devel/release.html)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge)](LICENSE)
[![MinIO](https://img.shields.io/badge/Storage-MinIO-C72E49?style=for-the-badge&logo=minio)](https://min.io/)

A high-performance Vector Search API built with Go, designed to efficiently store and search through document embeddings using semantic similarity. The API leverages MinIO for scalable object storage and implements concurrent processing for optimal performance.

## 🚀 Features

- **High-Performance Vector Search**: Optimized similarity search using cosine similarity
- **Concurrent Processing**: Parallel processing of documents and search operations
- **Scalable Storage**: MinIO integration for reliable and scalable object storage
- **Collection Management**: Create, search, and delete document collections
- **Multi-Collection Search**: Search across multiple collections simultaneously
- **Async Operations**: Non-blocking storage and deletion operations
- **Environment Configuration**: Secure credential management through environment variables

## 📋 Prerequisites

- Go 1.20 or higher
- MinIO instance (self-hosted or cloud service)
- Git

## 🛠️ Installation

1. Clone the repository:
```bash
git clone [https://github.com/yourusername/vector-search-api.git](https://github.com/dist-bit/nebuia_vector_db)
cd vector-search-api
```

2. Install dependencies:
```bash
go mod download
```

3. Create and configure your `.env` file:
```bash
cp .env.example .env
```

4. Update the `.env` file with your MinIO credentials:
```env
MINIO_ENDPOINT=your-minio-endpoint
MINIO_ACCESS_KEY=your-access-key
MINIO_SECRET_KEY=your-secret-key
MINIO_BUCKET_NAME=your-bucket-name
APP_PORT=5489
```

5. Build the application:
```bash
go build -o vector-search-api
```

## 🚦 Usage

### Starting the Server

```bash
./vector-search-api
```

The API will be available at `http://localhost:5489` (or your configured port).

### API Endpoints

#### Store Documents
```http
POST /store
Content-Type: application/json

{
    "collection_name": "my_collection",
    "documents": [
        {
            "text": "Document content",
            "metadata": {
                "source": "source_name",
                "name": "doc_name"
            },
            "chunks": [
                {
                    "text": "Chunk content",
                    "embedding": {
                        "vector": [0.1, 0.2, 0.3, ...]
                    },
                    "metadata": {
                        "source": "chunk_source",
                        "name": "chunk_name"
                    }
                }
            ]
        }
    ]
}
```

#### Search Documents
```http
POST /search
Content-Type: application/json

{
    "collection_name": "my_collection",
    "query_embedding": {
        "vector": [0.1, 0.2, 0.3, ...]
    },
    "top_k": 5
}
```

#### Multi-Collection Search
```http
POST /multi_search
Content-Type: application/json

{
    "collections": ["collection1", "collection2"],
    "query_embedding": {
        "vector": [0.1, 0.2, 0.3, ...]
    },
    "top_k": 5
}
```

#### Delete Collection
```http
POST /delete_collection
Content-Type: application/json

{
    "collection_name": "my_collection"
}
```

## 🏗️ Architecture

The API is built with the following components:

- **Fiber**: High-performance web framework
- **MinIO**: Distributed object storage
- **go-json**: Optimized JSON encoder/decoder
- **sync.Map**: Thread-safe concurrent map implementation
- **Context**: Context-based operation management
- **UUID**: Unique identifier generation

## 🔧 Configuration

The application can be configured through environment variables:

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| MINIO_ENDPOINT | MinIO server endpoint | Yes | - |
| MINIO_ACCESS_KEY | MinIO access key | Yes | - |
| MINIO_SECRET_KEY | MinIO secret key | Yes | - |
| MINIO_BUCKET_NAME | MinIO bucket name | Yes | - |
| APP_PORT | Application port | No | 5489 |

## ⚡ Performance Optimizations

- Concurrent document processing
- Connection pooling for MinIO operations
- Efficient vector similarity calculations
- Optimized JSON encoding/decoding
- Pre-allocated slices for search results

## 🔒 Security

- Secure credential management through environment variables
- TLS encryption for MinIO connections
- Request rate limiting and timeouts
- Input validation for all endpoints

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 📚 Documentation

For more detailed documentation, please refer to the [Wiki](https://github.com/yourusername/vector-search-api/wiki).

## 🙏 Acknowledgments

- [Fiber](https://github.com/gofiber/fiber)
- [MinIO](https://min.io/)
- [Go-JSON](https://github.com/goccy/go-json)
- [UUID](https://github.com/google/uuid)

## 📞 Support

For support, please open an issue in the GitHub repository or contact the maintainers.

---
Made with ❤️ by NebuIA

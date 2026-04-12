# DiveEnd PDF Processing Service

Python microservice for PDF parsing and LLM-based structured extraction.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    PDF Processing Service                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐   │
│  │   Health    │    │    Parse    │    │   Extract   │   │
│  │   Routes    │    │   Routes    │    │   Routes    │   │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘   │
│         │                  │                  │            │
│         └──────────────────┼──────────────────┘            │
│                            ▼                              │
│                   ┌─────────────────┐                      │
│                   │   Core Layer    │                      │
│                   │  - LLM Client   │                      │
│                   │  - PDF Parser   │                      │
│                   │  - Config       │                      │
│                   └─────────────────┘                      │
│                            │                              │
│         ┌──────────────────┼──────────────────┐          │
│         ▼                  ▼                  ▼          │
│  ┌────────────┐   ┌────────────┐   ┌──────────────┐   │
│  │   OpenAI   │   │  Anthropic │   │ PyMuPDF4LLM  │   │
│  │    API     │   │    API     │   │  (CPU Local) │   │
│  └────────────┘   └────────────┘   └──────────────┘   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Clone the repository
cd DiveEnd/services/pdf_service

# Set environment variables
export OPENAI_API_KEY=your_openai_key
export ANTHROPIC_API_KEY=your_anthropic_key

# Start services
docker-compose up -d

# Check health
curl http://localhost:50051/health/
```

### Local Development

```bash
# Create virtual environment
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Set environment variables
export OPENAI_API_KEY=your_openai_key
export ANTHROPIC_API_KEY=your_anthropic_key

# Run development server
uvicorn app.main:app --reload --host 0.0.0.0 --port 50051
```

## API Endpoints

### Health

```bash
# Basic health check
GET /health/

# Readiness check
GET /health/ready

# Liveness check
GET /health/live
```

### PDF Parsing

```bash
# Upload and parse PDF file
POST /parse/upload
Content-Type: multipart/form-data

file: <binary>
extract_sections: true

# Parse PDF from URL
POST /parse/url
Content-Type: application/json

{
  "url": "https://arxiv.org/pdf/xxxx.pdf",
  "extract_sections": true
}
```

### LLM Extraction

```bash
# Extract structured data from markdown
POST /extract/
Content-Type: application/json

{
  "markdown": "# Paper Title\n\n## Abstract...",
  "extraction_type": "all",
  "provider": "openai"
}
```

## Response Examples

### Parse Response

```json
{
  "success": true,
  "markdown": "# Paper Title\n\n## Abstract...",
  "metadata": {
    "title": "Paper Title",
    "authors": ["Author 1", "Author 2"],
    "pages": 10
  },
  "sections": ["Abstract", "Introduction", "Method", "Results", "Conclusion"],
  "error": null
}
```

### Extraction Response

```json
{
  "success": true,
  "data": {
    "metadata": {
      "title": "Advanced Robotics Control",
      "authors": ["John Doe", "Jane Smith"],
      "abstract": "This paper proposes...",
      "problem": "Current methods lack...",
      "method": "We propose a novel...",
      "github_url": "https://github.com/example/repo"
    },
    "metrics": [
      {
        "metric_name": "Success Rate",
        "dataset_or_task": "Humanoid Walking",
        "ours_value": "94.2%",
        "unit": "%"
      }
    ],
    "baselines": [
      {
        "metric_name": "Success Rate",
        "method_name": "Baseline Method",
        "value": "87.5%"
      }
    ],
    "relevance_tags": ["sim-to-real", "locomotion", "reinforcement-learning"]
  },
  "error": null,
  "provider": "openai",
  "model": "gpt-4o-mini"
}
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_API_KEY` | OpenAI API key | Required |
| `ANTHROPIC_API_KEY` | Anthropic API key | Required |
| `PDF_HOST` | Server host | `0.0.0.0` |
| `PDF_PORT` | Server port | `50051` |
| `PDF_WORKERS` | Number of workers | `1` |

## Architecture Decisions

1. **PyMuPDF4LLM for PDF Parsing**: Uses CPU-friendly markdown extraction and avoids downloading heavyweight local model weights such as `model.safetensors`.

2. **FastAPI over Flask/Django**: FastAPI provides async/await support, automatic OpenAPI documentation, and Pydantic validation - all critical for this service's performance and maintainability.

3. **Dual LLM Strategy**: Weak LLM (GPT-4o-mini) for speed and cost on simple tasks, Strong LLM (Claude Sonnet) for accuracy on complex extraction tasks.

4. **Docker Compose Setup**: Includes optional Redis for caching, making it production-ready with horizontal scaling potential.

## Development

### Running Tests

```bash
# Install dev dependencies
pip install pytest pytest-asyncio httpx

# Run tests
pytest tests/
```

### Code Style

```bash
# Format with black
black app/ core/

# Lint with ruff
ruff check app/ core/

# Type check with mypy
mypy app/ core/
```

## License

MIT License - See LICENSE file for details

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support

For issues and feature requests, please use the GitHub issue tracker.

---

**Version**: 0.1.0  
**Last Updated**: 2024-01-15  
**Maintainer**: DiveEnd Team

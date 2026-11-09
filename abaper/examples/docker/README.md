# Docker Deployment Example

This directory contains example files for deploying `abaper` using Docker.

## Quick Start

1. **Copy the example environment file:**
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env` with your SAP connection details:**
   ```bash
   SAP_HOST=your-sap-system.company.com:8000
   SAP_USERNAME=your-username
   SAP_PASSWORD=your-password
   ```

3. **Start the service:**
   ```bash
   docker-compose up -d
   ```

4. **Test the service:**
   ```bash
   curl http://localhost:8013/health
   ```

## Configuration

The example uses the published Docker image `ghcr.io/bluefunda/abaper:latest`.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SAP_HOST` | SAP system hostname and port | Required |
| `SAP_CLIENT` | SAP client number | `100` |
| `SAP_USERNAME` | SAP username | Required |
| `SAP_PASSWORD` | SAP password | Required |
| `LOG_LEVEL` | Logging level | `info` |

### Volumes

- `./config:/app/config:ro` - Configuration files (optional)
- `./logs:/app/logs` - Log files

## Production Considerations

- Use Docker secrets or a proper secret management system for passwords
- Set up proper logging and monitoring
- Configure resource limits based on your needs
- Consider using a reverse proxy (nginx, traefik) for HTTPS termination

## Stopping

```bash
docker-compose down
```

To remove all data:
```bash
docker-compose down -v
```

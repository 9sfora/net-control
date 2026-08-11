# NetControl

Plataforma modular para gerenciamento de equipamentos de rede. O primeiro módulo é o ACS.

## Backend local

Requisitos: Docker com Compose.

```bash
docker compose up --build
```

A API fica disponível em `http://localhost:8080`:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

Para executar os testes Go diretamente:

```bash
cd backend
go test ./...
```

As migrations em `postgres/migrations` são aplicadas automaticamente pelo serviço `migrate` antes da API e do worker iniciarem.

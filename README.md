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

No Compose, migrations e bootstrap usam o proprietário `netcontrol`; API e worker usam o papel limitado `netcontrol_app`. Essa separação é necessária porque proprietários/superusuários do PostgreSQL podem ignorar RLS. As credenciais presentes no Compose são exclusivas para desenvolvimento.

## Primeiro administrador

Após subir o ambiente, crie o primeiro tenant e administrador:

```bash
BOOTSTRAP_ADMIN_PASSWORD='uma-senha-segura' docker compose --profile tools run --rm bootstrap
```

O comando falha se o tenant já existir e nunca possui senha padrão persistida no projeto. Nome, slug e dados do administrador podem ser personalizados pelas variáveis `BOOTSTRAP_*` do Compose.

Autenticação:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"tenant":"netcontrol","email":"admin@netcontrol.local","password":"uma-senha-segura"}'
```

Use o `access_token` retornado como `Authorization: Bearer <token>`. O endpoint `GET /api/v1/auth/me` retorna a identidade e as permissões do usuário autenticado.

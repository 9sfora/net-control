# DESIGN — NetControl

## 1. Objetivo deste documento

Este documento registra as principais decisões de arquitetura e design técnico do **NetControl**.

O NetControl será uma plataforma modular para gerenciamento de equipamentos de rede de um provedor de internet. O primeiro módulo definido é o **ACS**, responsável pelo gerenciamento remoto de roteadores, firmwares, comandos, sessões e parâmetros dos equipamentos.

Este documento complementa:

- `PRD.md`: requisitos funcionais e de produto;
- `UML.md`: classes, entidades e relacionamentos;
- `DESIGN.md`: decisões técnicas e arquiteturais.

---

# 2. Princípios arquiteturais

O NetControl deverá seguir os seguintes princípios:

- arquitetura modular;
- modular monolith no início;
- baixo acoplamento entre módulos;
- separação clara entre Core e módulos de negócio;
- multi-tenancy desde o início;
- isolamento de dados por `tenant_id`;
- PostgreSQL Row Level Security;
- RBAC granular;
- API e workers separados;
- processamento assíncrono para operações remotas;
- idempotência;
- auditoria;
- observabilidade;
- segurança em múltiplas camadas;
- testes automatizados;
- infraestrutura preparada para crescimento;
- evitar dependência desnecessária de fabricantes específicos.

---

# 3. Stack tecnológica

## 3.1 Backend

Linguagem:

```text
Go
```

O backend será responsável por:

- API REST;
- autenticação;
- autorização;
- multi-tenancy;
- RBAC;
- integração com PostgreSQL;
- regras de negócio;
- processamento ACS;
- gerenciamento da fila de comandos;
- workers;
- auditoria;
- logging;
- error reporting;
- segurança da aplicação.

---

## 3.2 Frontend

Stack definida:

```text
React
TypeScript
Vite
Tailwind CSS
```

Motivos:

- aplicação administrativa autenticada;
- não há necessidade inicial de SSR;
- baixo acoplamento com o backend;
- build simples;
- boa experiência de desenvolvimento;
- forte ecossistema;
- facilidade para testes E2E.

---

## 3.3 Banco de dados

Banco:

```text
PostgreSQL
```

Principais recursos utilizados:

- schemas;
- UUID;
- JSONB;
- TIMESTAMPTZ;
- foreign keys;
- constraints;
- indexes;
- RLS;
- triggers;
- funções;
- transações;
- `FOR UPDATE SKIP LOCKED`.

---

# 4. Estrutura do repositório

Estrutura raiz definida:

```text
netcontrol/
├── postgres/
│   └── migrations/
│
├── backend/
│
├── frontend/
│
├── dockerfiles/
│
├── docs/
│   ├── DESIGN.md
│   ├── PRD.md
│   └── UML.md
│
├── README.md
├── docker-compose.yml
├── .gitignore
└── .dockerignore
```

---

# 5. Estrutura do backend

Estrutura inicial sugerida:

```text
backend/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── worker/
│       └── main.go
│
├── internal/
│   ├── core/
│   │   ├── auth/
│   │   ├── tenant/
│   │   ├── rbac/
│   │   ├── user/
│   │   ├── settings/
│   │   └── audit/
│   │
│   ├── acs/
│   │   ├── manufacturer/
│   │   ├── routermodel/
│   │   ├── firmware/
│   │   ├── router/
│   │   ├── wifi/
│   │   ├── command/
│   │   ├── session/
│   │   ├── blacklist/
│   │   ├── parameter/
│   │   └── event/
│   │
│   └── platform/
│       ├── database/
│       ├── config/
│       ├── logging/
│       ├── errors/
│       ├── middleware/
│       └── security/
│
├── pkg/
├── tests/
├── go.mod
└── go.sum
```

---

# 6. Estrutura do frontend

```text
frontend/
├── src/
│   ├── app/
│   ├── components/
│   ├── layouts/
│   ├── modules/
│   │   ├── auth/
│   │   ├── users/
│   │   ├── settings/
│   │   └── acs/
│   │       ├── dashboard/
│   │       ├── routers/
│   │       ├── models/
│   │       ├── firmwares/
│   │       ├── commands/
│   │       └── blacklist/
│   │
│   ├── services/
│   ├── hooks/
│   ├── types/
│   └── utils/
│
├── tests/
├── package.json
├── vite.config.ts
└── tsconfig.json
```

A organização do frontend deve refletir os módulos do backend.

---

# 7. Modular monolith

A arquitetura inicial será um **modular monolith**.

Isso significa:

- um repositório;
- um backend Go;
- módulos internos bem separados;
- banco compartilhado;
- schemas separados quando necessário;
- API e worker executados separadamente.

Exemplo:

```text
netcontrol-api
netcontrol-worker
```

Ambos compartilham o mesmo código de domínio.

Vantagens:

- menor complexidade operacional;
- desenvolvimento mais rápido;
- transações simples;
- fácil compartilhamento de contratos internos;
- possibilidade futura de extrair módulos para serviços separados.

---

# 8. Separação Core e módulos

## 8.1 Core

O Core do NetControl é independente dos módulos de negócio.

Responsabilidades:

```text
Authentication
Users
Tenants
RBAC
Settings
Audit
Security
Logging
Error reporting
```

Localização:

```text
backend/internal/core/
```

Banco:

```text
public
```

---

## 8.2 Módulo ACS

O ACS depende do Core.

Responsabilidades:

```text
Manufacturers
Router Models
Firmware
Routers
Wi-Fi
Sessions
Commands
Command Attempts
Command Events
Blacklist
Parameters
Events
```

Localização:

```text
backend/internal/acs/
```

Banco:

```text
acs
```

---

# 9. Dependência entre módulos

Regra:

```text
Core não depende de ACS.

ACS pode depender de Core.
```

Exemplo:

```text
public.users
       ↓
acs.router_commands.created_by
```

O caminho inverso deve ser evitado.

---

# 10. Schemas PostgreSQL

## 10.1 Schema `public`

O schema `public` será utilizado pelo núcleo do NetControl.

Principais tabelas:

```text
public.tenants
public.users
public.roles
public.permissions
public.user_roles
public.role_permissions
public.system_settings
public.audit_logs
```

---

## 10.2 Schema `acs`

Principais tabelas:

```text
acs.manufacturers
acs.router_models
acs.router_model_hardware_versions
acs.firmwares
acs.routers
acs.router_wifi_radios
acs.router_sessions
acs.router_commands
acs.router_command_attempts
acs.router_command_events
acs.router_blacklist
acs.router_parameters
acs.router_events
```

---

# 11. Multi-tenancy

Multi-tenancy é requisito fundamental.

Cada organização ou cliente lógico da plataforma será representado por um tenant.

Tabela:

```text
public.tenants
```

Entidades de negócio devem possuir:

```text
tenant_id UUID NOT NULL
```

Exemplos:

```text
acs.routers.tenant_id
acs.router_models.tenant_id
acs.firmwares.tenant_id
acs.router_commands.tenant_id
acs.router_blacklist.tenant_id
```

---

# 12. Identificação do tenant

O tenant será resolvido durante a autenticação ou contexto da requisição.

Fluxo:

```text
HTTP Request
    ↓
Authentication
    ↓
Resolve tenant
    ↓
Tenant Middleware
    ↓
Database transaction
    ↓
SET LOCAL app.tenant_id
    ↓
Query
```

---

# 13. PostgreSQL Row Level Security

O isolamento por `tenant_id` será reforçado com RLS.

Exemplo conceitual:

```sql
ALTER TABLE acs.routers ENABLE ROW LEVEL SECURITY;

CREATE POLICY routers_tenant_isolation
ON acs.routers
USING (
    tenant_id = current_setting('app.tenant_id')::uuid
);
```

Durante a transação:

```sql
SET LOCAL app.tenant_id = '<tenant_uuid>';
```

Benefício:

mesmo que uma query seja escrita incorretamente sem filtro de tenant, o PostgreSQL restringirá as linhas retornadas.

---

# 14. Política de RLS

Por padrão:

- todas as tabelas tenant-scoped terão RLS;
- acesso sem contexto de tenant deverá falhar;
- workers deverão configurar explicitamente o tenant;
- migrations e operações administrativas terão mecanismos controlados de bypass;
- testes de integração deverão validar isolamento entre tenants.

---

# 15. RBAC

O NetControl utilizará RBAC granular.

Entidades principais:

```text
User
Role
Permission
UserRole
RolePermission
```

Não utilizar apenas níveis numéricos fixos.

Permissões devem representar ações.

Exemplos:

```text
acs.router.read
acs.router.create
acs.router.update
acs.router.delete

acs.router.command.reboot
acs.router.command.activate
acs.router.command.block
acs.router.command.update_firmware

acs.firmware.read
acs.firmware.create
acs.firmware.upload

acs.blacklist.read
acs.blacklist.create
acs.blacklist.delete

system.users.read
system.users.manage

system.settings.read
system.settings.manage
```

---

# 16. Matriz de acesso

Exemplo:

| Permissão | Admin | ACS Operator | Viewer |
|---|---:|---:|---:|
| `acs.router.read` | ✓ | ✓ | ✓ |
| `acs.router.create` | ✓ | ✓ | |
| `acs.router.update` | ✓ | ✓ | |
| `acs.router.command.reboot` | ✓ | ✓ | |
| `acs.router.command.block` | ✓ | ✓ | |
| `acs.firmware.create` | ✓ | | |
| `acs.blacklist.create` | ✓ | ✓ | |
| `system.users.manage` | ✓ | | |
| `system.settings.manage` | ✓ | | |

A matriz deve ser configurável por tenant.

---

# 17. RBAC e RLS

Os dois mecanismos têm objetivos diferentes.

```text
RBAC
↓
O usuário pode executar esta ação?
```

```text
RLS
↓
O usuário pode acessar esta linha?
```

Exemplo:

```text
Usuário possui:
acs.router.update

Tenant do usuário:
A

Roteador:
Tenant B

Resultado:
acesso negado pelo RLS
```

---

# 18. API

A API será versionada.

Base:

```text
/api/v1
```

Core:

```text
/api/v1/auth
/api/v1/users
/api/v1/roles
/api/v1/tenants
/api/v1/settings
```

ACS:

```text
/api/v1/acs/routers
/api/v1/acs/router-models
/api/v1/acs/firmwares
/api/v1/acs/commands
/api/v1/acs/blacklist
```

---

# 19. API de comandos

Exemplo:

```text
POST /api/v1/acs/routers/{router_id}/commands
```

Payload:

```json
{
  "type": "REBOOT"
}
```

A API não executará diretamente o comando no roteador.

Ela deverá:

```text
validar usuário
↓
validar tenant
↓
validar RBAC
↓
validar roteador
↓
criar RouterCommand
↓
retornar comando criado
```

---

# 20. Processamento assíncrono

Operações remotas utilizarão fila persistida no PostgreSQL.

Componentes:

```text
API
Worker
PostgreSQL
ACS
```

Fluxo:

```text
API
↓
INSERT router_commands
↓
PENDING

Worker
↓
adquire comando
↓
PROCESSING
↓
executa
↓
COMPLETED / RETRY_SCHEDULED / FAILED
```

---

# 21. Worker

O worker será executado separadamente:

```text
backend/cmd/worker
```

A API:

```text
backend/cmd/api
```

Em containers:

```text
netcontrol-api
netcontrol-worker
```

---

# 22. Concorrência da fila

Para permitir múltiplos workers:

```sql
SELECT ...
FROM acs.router_commands
WHERE ...
FOR UPDATE SKIP LOCKED;
```

Isso impede que dois workers adquiram o mesmo comando simultaneamente.

---

# 23. Ciclo de vida dos comandos

Estados:

```text
PENDING
SCHEDULED
WAITING_DEVICE
PROCESSING
RETRY_SCHEDULED
COMPLETED
FAILED
CANCELED
EXPIRED
```

Estados finais:

```text
COMPLETED
FAILED
CANCELED
EXPIRED
```

---

# 24. Retentativas

Configuração padrão inicial:

```text
Tentativa 1: imediata
Tentativa 2: +1 minuto
Tentativa 3: +5 minutos
Tentativa 4: +15 minutos
Tentativa 5: +1 hora
```

Configuração deverá poder ser alterada.

---

# 25. Idempotência

Comandos deverão suportar:

```text
idempotency_key
```

Exemplos:

```text
activate-router:<router_id>

block-router:<router_id>

update-firmware:<router_id>:<firmware_id>
```

Objetivo:

evitar duplicação causada por:

- retries HTTP;
- clique duplicado;
- falha de rede;
- integração externa;
- workers.

---

# 26. Estado desejado e reportado

Roteadores devem possuir estado desejado e estado reportado.

```text
desired_status
reported_status
```

Exemplo:

```text
desired_status = BLOCKED
reported_status = ACTIVE
```

Significa que existe alteração pendente.

Quando confirmada:

```text
desired_status = BLOCKED
reported_status = BLOCKED
```

---

# 27. Conectividade dos roteadores

O status online não deverá ser persistido como simples boolean.

Deve ser calculado por:

```text
last_seen_at
```

Estados iniciais:

```text
ONLINE
STALE
OFFLINE
NEVER_CONNECTED
```

Parâmetros configuráveis:

```text
acs.online_timeout_minutes
acs.offline_timeout_hours
```

---

# 28. Secrets Management

Variáveis sensíveis serão fornecidas por environment variables.

Desenvolvimento:

```text
.env
```

O `.env` nunca deverá ser commitado.

Repositorio:

```text
.env.example
```

Exemplos:

```text
DATABASE_URL
JWT_SECRET
WIFI_ENCRYPTION_KEY
SENTRY_DSN
ACS_PRIVATE_KEY
```

---

# 29. Secrets em produção

A aplicação Go não deve depender de um gerenciador específico.

Ela lê apenas environment variables.

Isso permite utilizar:

```text
Docker Secrets
Kubernetes Secrets
HashiCorp Vault
AWS Secrets Manager
GCP Secret Manager
```

sem alterar regras de negócio.

---

# 30. Configuração da aplicação

Configurações não sensíveis podem ser armazenadas em:

```text
public.system_settings
```

Exemplos:

```text
system.name
system.timezone
system.language

acs.online_timeout_minutes
acs.offline_timeout_hours
acs.command_max_attempts
acs.command_expiration_hours
acs.command_retry_intervals
```

Secrets não deverão ser armazenados nesta tabela.

---

# 31. Segurança da senha Wi-Fi

A senha de Wi-Fi precisa ser recuperada para envio ao equipamento.

Portanto:

- hash não é suficiente;
- deverá utilizar criptografia reversível;
- a chave deverá ser fornecida via secret externo;
- a chave não deverá estar armazenada no mesmo banco.

---

# 32. Logging

Logs deverão ser estruturados.

Formato preferido:

```text
JSON
```

Exemplo:

```json
{
  "level": "error",
  "module": "acs",
  "tenant_id": "...",
  "router_id": "...",
  "command_id": "...",
  "request_id": "...",
  "error": "connection timeout"
}
```

---

# 33. Contexto de logs

Sempre que aplicável, incluir:

```text
request_id
trace_id
tenant_id
user_id
module
router_id
command_id
```

---

# 34. Error Reporting

Logging e error reporting são conceitos separados.

Logging:

```text
registro operacional
```

Error reporting:

```text
captura e agrupamento de falhas inesperadas
```

A solução deverá permitir integração com plataformas como:

```text
Sentry
```

ou equivalente.

---

# 35. Audit Log

Auditoria é diferente de logs técnicos.

Exemplo:

```text
Usuário executou BLOCK em Router X
```

Tabela:

```text
public.audit_logs
```

Campos sugeridos:

```text
id
tenant_id
user_id
module
action
entity_type
entity_id
old_data
new_data
ip_address
user_agent
created_at
```

---

# 36. Operações auditáveis

Pelo menos:

- login relevante;
- criação de usuário;
- alteração de role;
- mudança de permissão;
- cadastro de roteador;
- edição de roteador;
- execução de comando;
- cancelamento de comando;
- cadastro de firmware;
- upload de firmware;
- inclusão em blacklist;
- remoção de blacklist;
- alteração de configurações.

---

# 37. Testes

O projeto deverá possuir três níveis principais.

```text
Unit
Integration
E2E
```

---

# 38. Testes unitários

Backend Go:

- regras de domínio;
- serviços;
- validações;
- RBAC;
- state machine;
- retry;
- idempotência;
- cálculo de status.

Exemplo:

```text
PROCESSING → COMPLETED = permitido
COMPLETED → PROCESSING = proibido
```

---

# 39. Testes de integração

Utilizar PostgreSQL real ou container isolado.

Cobrir:

- repositories;
- migrations;
- constraints;
- transações;
- RLS;
- multi-tenancy;
- fila de comandos;
- locking;
- idempotência.

Teste obrigatório:

```text
Tenant A cria Router A.

Tenant B consulta Router A.

Resultado:
nenhuma linha acessível.
```

---

# 40. Testes E2E

Ferramenta:

```text
Playwright
```

Exemplo:

```text
Login
↓
abrir roteador
↓
executar Reiniciar
↓
confirmar
↓
comando aparece na fila
```

---

# 41. Security Audit

A segurança deverá fazer parte do CI.

Pipeline conceitual:

```text
Push
↓
Lint
↓
Unit Tests
↓
Integration Tests
↓
Security Scan
↓
Dependency Audit
↓
Build
↓
E2E
```

---

# 42. Verificações de segurança

Incluir:

```text
SAST
Dependency scanning
Secret scanning
Container scanning
Go security analysis
Frontend dependency audit
```

---

# 43. WAF

Produção deverá possuir camada de proteção antes da aplicação.

Arquitetura:

```text
Internet
↓
WAF
↓
Bot Protection
↓
Rate Limiting
↓
Reverse Proxy
↓
NetControl
```

---

# 44. Bot Fight Mode

Quando utilizado Cloudflare ou solução equivalente, habilitar proteção contra tráfego automatizado malicioso.

Objetivo:

- reduzir scans;
- reduzir bots;
- dificultar abuso automatizado;
- reduzir carga indevida na aplicação.

---

# 45. Rate Limiting

Não utilizar apenas limite global.

Aplicar regras específicas por tipo de endpoint.

Exemplo:

```text
/login
5 req/min/IP
```

API geral:

```text
/api/*
300 req/min/user
```

Operações críticas:

```text
POST /routers/{id}/commands
```

deverão possuir regras adicionais.

---

# 46. Proteção lógica contra abuso

Além do rate limit HTTP, regras de negócio deverão impedir abuso.

Exemplo:

não permitir criação indiscriminada de centenas de comandos `REBOOT` simultâneos para o mesmo roteador.

---

# 47. TLS/SSL

Todo tráfego em produção deverá utilizar HTTPS.

```text
HTTP
↓
301/308
↓
HTTPS
```

Versão mínima recomendada:

```text
TLS 1.2+
```

---

# 48. HSTS

Resposta deverá incluir:

```http
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

`preload` somente deverá ser habilitado após validação de todos os subdomínios.

---

# 49. Security Headers

Além de HSTS, considerar:

```text
Content-Security-Policy
X-Content-Type-Options
Referrer-Policy
Permissions-Policy
frame-ancestors via CSP
```

---

# 50. CORS

CORS deverá ser restritivo.

Não utilizar:

```text
Access-Control-Allow-Origin: *
```

em endpoints autenticados.

A origem do frontend deverá ser explicitamente configurada.

---

# 51. Timeouts

Serviços HTTP deverão possuir:

- read timeout;
- write timeout;
- idle timeout;
- request timeout;
- limites de payload.

Evitar conexões ilimitadas.

---

# 52. Docker

Docker será utilizado no ambiente de desenvolvimento e deploy.

Arquivos:

```text
dockerfiles/backend.Dockerfile
dockerfiles/worker.Dockerfile
dockerfiles/frontend.Dockerfile
```

Orquestração local:

```text
docker-compose.yml
```

---

# 53. Containers iniciais

Sugestão:

```text
postgres
backend
worker
frontend
```

Possíveis serviços futuros:

```text
redis
reverse-proxy
observability
```

Redis não é requisito inicial para a fila, pois a fila será persistida no PostgreSQL.

---

# 54. Migrations

Local:

```text
postgres/migrations/
```

As migrations devem ser pequenas e organizadas por mudança lógica.

Exemplo:

```text
000001_extensions
000002_core_tenants
000003_core_users
000004_core_rbac
000005_core_settings
000006_core_audit
000007_acs_schema
000008_acs_manufacturers
000009_acs_router_models
000010_acs_firmwares
000011_acs_routers
000012_acs_wifi
000013_acs_commands
000014_acs_blacklist
000015_acs_rls
```

Preferência por migrations:

```text
.up.sql
.down.sql
```

---

# 55. Banco como camada de segurança

O PostgreSQL não deverá ser tratado apenas como armazenamento passivo.

Ele será utilizado para:

- foreign keys;
- constraints;
- uniqueness;
- RLS;
- locks;
- transactions;
- indexes;
- integridade;
- concurrency control.

A aplicação não deverá ser a única responsável pela integridade.

---

# 56. IDs

Entidades principais utilizarão:

```text
UUID
```

Geração:

```text
gen_random_uuid()
```

via extensão `pgcrypto`.

---

# 57. Datas

Utilizar:

```text
TIMESTAMPTZ
```

para datas relevantes.

A aplicação deve trabalhar internamente de forma timezone-safe.

Timezone padrão de apresentação será configurável.

---

# 58. JSONB

Utilizar JSONB para dados variáveis.

Exemplos:

```text
command.parameters
command.result
session.metadata
event.payload
event.metadata
audit.old_data
audit.new_data
```

Não utilizar JSONB como substituto de modelagem relacional para campos estáveis.

---

# 59. API e domínio

Handlers HTTP não devem conter regras de negócio significativas.

Fluxo preferido:

```text
Handler
↓
Service / Use Case
↓
Domain
↓
Repository
↓
PostgreSQL
```

---

# 60. Middleware

Middlewares globais sugeridos:

```text
Request ID
Authentication
Tenant Resolution
RBAC
Rate Limiting
Security Headers
Recovery
Structured Logging
```

---

# 61. Ordem da segurança da requisição

Fluxo conceitual:

```text
Request
↓
Request ID
↓
Rate Limit
↓
Authentication
↓
Tenant Context
↓
RBAC
↓
Handler
↓
Service
↓
Database + RLS
```

---

# 62. Tratamento de erros da API

Erros deverão possuir formato consistente.

Exemplo:

```json
{
  "error": {
    "code": "ACS_ROUTER_NOT_FOUND",
    "message": "Router not found",
    "request_id": "..."
  }
}
```

Não retornar:

- stack traces;
- SQL;
- secrets;
- detalhes internos sensíveis.

---

# 63. Tipos de erro

Categorias sugeridas:

```text
VALIDATION_ERROR
UNAUTHORIZED
FORBIDDEN
NOT_FOUND
CONFLICT
RATE_LIMITED
INTERNAL_ERROR
DEPENDENCY_ERROR
```

---

# 64. Frontend e permissões

O frontend poderá esconder ações não permitidas.

Exemplo:

```text
Usuário sem acs.router.command.block
↓
botão "Bloquear" não aparece
```

Entretanto, o frontend nunca será considerado camada de segurança.

A API sempre deverá validar RBAC.

---

# 65. Fluxo de autenticação

Fluxo inicial:

```text
Login
↓
Backend valida usuário
↓
Sessão/JWT emitido
↓
Tenant selecionado/resolvido
↓
Frontend envia credencial
↓
Backend valida contexto
```

A estratégia exata de sessão/JWT poderá ser definida durante a implementação.

---

# 66. Observabilidade

O sistema deverá permitir observar:

- erros;
- latência;
- requests;
- workers;
- comandos;
- tentativas;
- filas;
- conexões ACS;
- falhas de banco.

---

# 67. Métricas sugeridas

```text
http_requests_total
http_request_duration_seconds

acs_router_sessions_total
acs_router_last_seen_age_seconds

acs_commands_created_total
acs_commands_completed_total
acs_commands_failed_total
acs_commands_pending

acs_command_attempts_total

worker_jobs_processed_total
worker_job_duration_seconds
```

---

# 68. Health checks

Endpoints sugeridos:

```text
/health
/ready
```

`/health`:

processo está ativo.

`/ready`:

dependências essenciais estão disponíveis.

---

# 69. Módulo ACS

O módulo ACS deverá permanecer desacoplado de fabricante específico.

Suportar árvores de parâmetros diferentes:

```text
Device.*
```

e:

```text
InternetGatewayDevice.*
```

Diferenças deverão ser tratadas por adapters, profiles ou mappings do módulo.

---

# 70. Extensibilidade do ACS

A arquitetura deverá permitir futuramente:

```text
TR-369 / USP
Router Groups
Configuration Templates
Firmware Campaigns
Monitoring Profiles
Diagnostics
Connected Clients
WAN Metrics
Wi-Fi Metrics
Zero-Touch Provisioning
```

---

# 71. Direção futura

Se algum módulo atingir necessidade operacional própria, poderá ser extraído para um serviço independente.

Exemplo futuro:

```text
NetControl Core
NetControl ACS Service
NetControl Monitoring Service
```

Essa extração não deverá ser necessária no MVP.

---

# 72. Decisões oficiais atuais

Resumo das decisões técnicas definidas até o momento:

```text
Backend:
Go

Frontend:
React + TypeScript + Vite + Tailwind CSS

Database:
PostgreSQL

Architecture:
Modular Monolith

Multi-tenancy:
tenant_id

Database isolation:
PostgreSQL RLS

Authorization:
RBAC granular

Secrets:
Environment variables / .env apenas local

Queue:
PostgreSQL persisted queue

Concurrency:
FOR UPDATE SKIP LOCKED

Workers:
Processo separado da API

Error Reporting:
Obrigatório

Logging:
Estruturado

Audit:
Obrigatório

Testing:
Unit + Integration + E2E

E2E:
Playwright

Security:
Security Audit + SAST + Dependency Scan + Secret Scan

Edge Security:
WAF + Bot Fight Mode + Rate Limiting

Transport Security:
TLS/SSL + HSTS

Repository:
Monorepo com backend, frontend, postgres, dockerfiles e docs
```

---

# 73. Premissas para novas funcionalidades

Toda nova funcionalidade do NetControl deverá responder às seguintes perguntas:

1. pertence ao Core ou a um módulo?
2. os dados são globais ou pertencem a um tenant?
3. a tabela precisa de RLS?
4. quais permissões RBAC são necessárias?
5. a operação precisa ser auditada?
6. precisa ser assíncrona?
7. precisa de idempotência?
8. quais testes unitários são necessários?
9. quais testes de integração são necessários?
10. existe impacto de segurança?
11. existe impacto em observabilidade?
12. existe impacto no isolamento multi-tenant?

Essas perguntas devem orientar a evolução do projeto.
# NetControl

Plataforma modular para gerenciamento de equipamentos de rede. O primeiro módulo é o ACS.

## Backend local

Requisitos: Docker com Compose.

```bash
docker compose up --build
```

A API fica disponível em `http://localhost:8080`:

```bash
curl -H 'Host: localhost' http://localhost:8080/health
curl -H 'Host: localhost' http://localhost:8080/ready
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

## FQDNs

O mesmo processo atende dois hosts lógicos, configurados sem protocolo ou caminho:

```text
APPLICATION_FQDN=net-control.dominio.com
ACS_FQDN=acs.dominio.com
ACS_DEFAULT_TENANT_SLUG=netcontrol
```

O host da aplicação expõe a API administrativa autenticada. O host ACS será exclusivo para sessões iniciadas pelos equipamentos. Requisições para hosts desconhecidos recebem `421`, e rotas administrativas não existem no host ACS.

O endpoint inicial dos equipamentos é `POST /cwmp`. O slug padrão associa essa URL fixa ao tenant responsável pelos equipamentos descobertos. Um `Inform` válido cria ou atualiza a entrada em `acs.unknown_routers`; se a identidade já existir em `acs.routers`, atualiza o equipamento configurado. Cada `Inform` também abre um registro em `acs.router_sessions`.

Rotas administrativas de descoberta:

```text
GET  /api/v1/acs/unknown-routers?limit=50&offset=0
POST /api/v1/acs/unknown-routers/{id}/configure
```

Payload da configuração inicial:

```json
{
  "name": "Roteador do cliente"
}
```

A promoção move a identidade para `acs.routers` e preserva as sessões recebidas enquanto o equipamento ainda era desconhecido.

Configuração desejada e ativação:

```text
PUT  /api/v1/acs/routers/{id}/configuration
POST /api/v1/acs/routers/{id}/commands
```

Exemplo de configuração:

```json
{
  "wifi": [
    {
      "band": "2.4GHZ",
      "ssid": "Cliente",
      "password": "senha-segura",
      "security_mode": "WPA2_PSK",
      "channel_mode": "AUTO",
      "enabled": true
    }
  ],
  "network": {
    "dhcp_enabled": true,
    "dhcp_start": "192.168.1.100",
    "dhcp_end": "192.168.1.200",
    "dhcp_lease_seconds": 86400,
    "dns_primary": "1.1.1.1",
    "dns_secondary": "1.0.0.1"
  }
}
```

Para enfileirar a ativação:

```json
{
  "type": "ACTIVATE"
}
```

O header opcional `Idempotency-Key` evita comandos duplicados. `WIFI_ENCRYPTION_KEY` deve conter uma chave aleatória de 32 bytes codificada em Base64; o valor padrão do Compose serve somente para desenvolvimento.

Na sessão CWMP de um roteador conhecido, o fluxo de ativação é:

```text
Inform → InformResponse → POST vazio → SetParameterValues
       → SetParameterValuesResponse → comando COMPLETED
```

O claim do comando usa `FOR UPDATE SKIP LOCKED`, cria uma entrada em `router_command_attempts` e registra transições em `router_command_events`. Um `SOAP Fault` marca tentativa e comando como `FAILED`, preservando código e mensagem retornados pelo CPE. O profile inicial gera parâmetros TR-181 (`Device.*`); mappings específicos por fabricante/modelo serão adicionados separadamente.

## Manutenção da fila

O worker percorre os tenants ativos e executa a manutenção da fila dentro de transações com RLS:

- move comandos novos para `WAITING_DEVICE`;
- marca prazos vencidos como `EXPIRED`;
- recupera comandos presos em `PROCESSING`;
- agenda `RETRY_SCHEDULED` com backoff;
- marca `FAILED` quando `max_attempts` é atingido;
- encerra a tentativa travada e registra cada transição.

Configuração:

```env
WORKER_INTERVAL=1s
COMMAND_PROCESSING_TIMEOUT=5m
COMMAND_RETRY_INTERVALS=30s,2m,10m
```

## API da fila

```text
GET  /api/v1/acs/commands?status=WAITING_DEVICE&router_id={uuid}&limit=50&offset=0
GET  /api/v1/acs/commands/{id}
POST /api/v1/acs/commands/{id}/cancel
```

O detalhe inclui tentativas e eventos em ordem cronológica. O snapshot sensível da configuração não é exposto. Apenas comandos em `PENDING`, `WAITING_DEVICE` ou `RETRY_SCHEDULED` podem ser cancelados; estados em execução ou finais retornam conflito.

## API de roteadores

```text
GET /api/v1/acs/routers
GET /api/v1/acs/routers/{id}
```

A listagem aceita `search`, `connectivity_status`, `desired_status`, `limit` e `offset`. O detalhe reúne configuração Wi-Fi mascarada, rede, as 20 sessões mais recentes e os 20 comandos mais recentes.

A conectividade é calculada a partir de `last_seen_at`, sem boolean persistido:

```env
ACS_ONLINE_TIMEOUT=10m
ACS_OFFLINE_TIMEOUT=24h
```

Os estados possíveis são `ONLINE`, `STALE`, `OFFLINE` e `NEVER_CONNECTED`.

## Dashboard ACS

```text
GET /api/v1/acs/dashboard
```

Retorna, em um snapshot consistente por tenant:

- totais de roteadores por conectividade e bloqueio;
- comandos pendentes, aguardando dispositivo, processando, em retentativa, falhos e expirados;
- comandos concluídos nas últimas 24 horas;
- dez conexões recentes;
- dez atividades recentes da fila;
- dez falhas recentes.

A rota exige a permissão `acs.dashboard.read`.

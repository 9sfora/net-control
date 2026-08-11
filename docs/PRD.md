# PRD — NetControl

## 1. Visão geral

**Produto:** NetControl  
**Módulo:** ACS  
**Status do documento:** Draft  
**Versão:** 1.0  
**Data:** 2026-08-10

O **NetControl** é uma plataforma para gerenciamento de equipamentos de rede utilizados por provedores de internet.

O sistema será composto por um núcleo principal e módulos especializados. O núcleo do NetControl será responsável por funcionalidades compartilhadas, como usuários, permissões, configurações e auditoria. Cada módulo poderá possuir seu próprio domínio funcional e schema no banco de dados.

O primeiro módulo definido é o **ACS**, responsável pelo gerenciamento remoto de roteadores e equipamentos compatíveis com protocolos de auto-configuração, incluindo TR-069.

## 2. Objetivo do produto

Centralizar o gerenciamento de equipamentos de rede de clientes em uma única plataforma, permitindo:

- cadastrar e organizar modelos de roteadores;
- cadastrar e distribuir firmwares;
- registrar e acompanhar roteadores;
- visualizar o estado e a última comunicação dos equipamentos;
- gerenciar configurações Wi-Fi;
- enviar comandos remotos;
- processar comandos de forma assíncrona;
- realizar novas tentativas automaticamente;
- manter histórico completo do ciclo de vida dos comandos;
- bloquear equipamentos por blacklist;
- acompanhar eventos e parâmetros reportados pelo roteador;
- permitir expansão futura para outros módulos do NetControl.

## 3. Arquitetura lógica

O banco PostgreSQL será organizado por schemas.

### 3.1 Schema `public`

O schema `public` representa o núcleo do NetControl.

Responsabilidades:

- usuários;
- autenticação;
- perfis;
- permissões;
- configurações globais;
- auditoria;
- informações gerais do sistema.

Tabelas sugeridas:

```text
public.users
public.roles
public.permissions
public.user_roles
public.role_permissions
public.system_settings
public.audit_logs
```

### 3.2 Schema `acs`

O schema `acs` contém exclusivamente as entidades relacionadas ao módulo ACS.

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

## 4. Perfis de usuário

### 4.1 Administrador

Pode:

- gerenciar usuários;
- alterar configurações do NetControl;
- alterar configurações do ACS;
- cadastrar modelos;
- cadastrar firmwares;
- cadastrar roteadores;
- executar qualquer comando;
- consultar históricos;
- gerenciar blacklist.

### 4.2 Operador ACS

Pode:

- consultar roteadores;
- cadastrar ou editar roteadores;
- executar comandos autorizados;
- consultar fila de comandos;
- consultar histórico;
- visualizar firmwares e modelos;
- gerenciar blacklist conforme permissão.

### 4.3 Visualizador

Pode:

- visualizar roteadores;
- consultar modelos;
- consultar firmwares;
- consultar comandos e históricos;
- não pode realizar alterações.

O modelo de permissões deve ser baseado em RBAC.

## 5. Dashboard do ACS

O dashboard deve apresentar:

- total de roteadores;
- roteadores online;
- roteadores offline;
- roteadores bloqueados;
- roteadores nunca conectados;
- comandos pendentes;
- comandos aguardando dispositivo;
- comandos em processamento;
- comandos com falha;
- comandos concluídos nas últimas 24 horas;
- equipamentos com conexão recente;
- atividade recente da fila;
- falhas recentes;
- atualizações de firmware recentes.

## 6. Fabricantes

O ACS deve permitir cadastrar fabricantes.

Campos:

- ID;
- nome;
- descrição;
- ativo;
- data de criação;
- data de atualização.

Exemplos:

```text
TP-Link
Huawei
Intelbras
ZTE
Nokia
```

## 7. Modelos de roteadores

O sistema deve permitir cadastrar os modelos de equipamentos suportados.

Campos:

- nome;
- nome utilizado pelo fabricante;
- fabricante;
- suporte a Wi-Fi 2.4 GHz;
- suporte a Wi-Fi 5 GHz;
- modos de autenticação suportados;
- versão de hardware;
- Product Class;
- descrição;
- OUI;
- protocolo suportado;
- ativo;
- data de criação;
- data de atualização.

Um mesmo modelo poderá possuir múltiplas versões de hardware.

Exemplo:

```text
Archer C6
├── V2
├── V3
└── V3.20
```

## 8. Firmware

O sistema deve permitir manter um catálogo de firmwares.

Campos:

- nome;
- modelo de roteador;
- versão de hardware compatível;
- versão do firmware;
- descrição;
- arquivo;
- nome original do arquivo;
- tamanho do arquivo;
- checksum;
- release notes;
- ativo;
- firmware padrão;
- status;
- data de upload;
- data de criação;
- data de atualização.

O firmware poderá ser cadastrado sem arquivo. Nesse caso:

```text
status = SEM_ARQUIVO
```

O arquivo poderá ser anexado posteriormente.

Após o upload deverá ser calculado checksum SHA-256.

## 9. Roteadores

O roteador representa uma instância física de equipamento.

### 9.1 Dados cadastrais

- nome;
- modelo;
- número de série;
- MAC address;
- UPnP ativo ou desativado;
- descrição.

### 9.2 Identificação ACS

- fabricante;
- OUI;
- Product Class;
- número de série reportado;
- versão de hardware;
- versão de software.

### 9.3 Conectividade

O sistema deve armazenar:

```text
first_seen_at
last_seen_at
last_inform_at
last_ip_address
last_session_id
last_contact_status
```

## 10. Estado de conectividade

O estado online não deve depender de um boolean persistido. Ele deve ser calculado utilizando `last_seen_at`.

Configuração inicial sugerida:

```text
ONLINE
Última comunicação menor que 10 minutos.

STALE
Última comunicação entre 10 minutos e 24 horas.

OFFLINE
Última comunicação superior a 24 horas.

NEVER_CONNECTED
Equipamento nunca se conectou.
```

Os intervalos deverão ser configuráveis.

## 11. Estado desejado e reportado

O NetControl deverá distinguir o estado solicitado pelo sistema do estado confirmado pelo equipamento.

```text
desired_status
reported_status
```

Exemplo:

```text
desired_status = BLOCKED
reported_status = ACTIVE
```

Isso significa que existe uma solicitação ainda não aplicada.

## 12. Wi-Fi

As configurações de Wi-Fi 2.4 GHz e 5 GHz deverão compartilhar a mesma estrutura.

Bandas iniciais:

```text
2_4_GHZ
5_GHZ
```

A arquitetura deverá permitir futuramente:

```text
6_GHZ
```

Campos:

- SSID;
- limite mínimo de RSSI em dBm;
- modo;
- segurança;
- senha;
- modo de autenticação;
- canal;
- modo de canal automático ou manual;
- largura de banda;
- potência;
- ativo;
- visível;
- isolamento de clientes;
- ATF.

A senha Wi-Fi deve utilizar criptografia reversível, pois precisa ser recuperada para envio ao roteador. A chave de criptografia não deve ficar no banco.

## 13. Comandos remotos

Comandos inicialmente definidos:

```text
REBOOT
ACTIVATE
BLOCK
UPDATE_FIRMWARE
ENABLE_ANTI_THEFT
DISABLE_ANTI_THEFT
ENABLE_MONITORING
DISABLE_MONITORING
```

## 14. Fila de comandos

Todos os comandos devem ser processados de forma assíncrona.

Fluxo básico:

```text
Operador solicita ação
↓
Comando é criado em acs.router_commands
↓
Worker identifica o comando
↓
Executa imediatamente ou aguarda o dispositivo
↓
Registra tentativa
↓
Conclui, agenda retentativa ou falha
```

## 15. Ciclo de vida do comando

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

Fluxo de sucesso:

```text
PENDING
↓
WAITING_DEVICE
↓
PROCESSING
↓
COMPLETED
```

Fluxo com retentativa:

```text
PROCESSING
↓
RETRY_SCHEDULED
↓
WAITING_DEVICE
↓
PROCESSING
↓
COMPLETED
```

## 16. Dados da fila de comandos

Cada comando deve armazenar:

- ID;
- roteador;
- tipo;
- parâmetros JSONB;
- status;
- prioridade;
- quantidade de tentativas;
- máximo de tentativas;
- data de disponibilidade;
- agendamento;
- início de processamento;
- última tentativa;
- próxima tentativa;
- conclusão;
- expiração;
- cancelamento;
- código do último erro;
- mensagem do último erro;
- resultado final;
- usuário responsável;
- data de criação;
- data de atualização.

## 17. Tentativas de execução

Cada execução deverá gerar um registro individual em:

```text
acs.router_command_attempts
```

Campos:

- ID;
- comando;
- número da tentativa;
- status;
- início;
- fim;
- sessão ACS;
- request;
- response;
- código de erro;
- mensagem de erro;
- duração em milissegundos;
- data de criação.

Status:

```text
STARTED
SUCCESS
FAILED
TIMEOUT
CONNECTION_LOST
REJECTED
```

## 18. Histórico do ciclo de vida

Mudanças de estado devem ser registradas em:

```text
acs.router_command_events
```

Campos:

- ID;
- comando;
- status anterior;
- novo status;
- motivo;
- metadata;
- usuário ou serviço responsável;
- data do evento.

## 19. Retentativas

Política inicial sugerida:

```text
1ª tentativa: imediata
2ª tentativa: +1 minuto
3ª tentativa: +5 minutos
4ª tentativa: +15 minutos
5ª tentativa: +1 hora
```

Se a execução depender de uma nova sessão do equipamento, o comando poderá permanecer como:

```text
WAITING_DEVICE
```

Um novo `Inform` poderá disparar o processamento.

## 20. Idempotência

A fila deve evitar comandos duplicados indevidos.

Campo:

```text
idempotency_key
```

Exemplos:

```text
activate-router:<router_id>
block-router:<router_id>
update-firmware:<router_id>:<firmware_id>
```

Não deve existir mais de uma atualização de firmware ativa para o mesmo roteador.

## 21. Prioridade

Sugestão:

```text
LOW
NORMAL
HIGH
CRITICAL
```

Ordenação do worker:

1. prioridade;
2. horário de disponibilidade;
3. horário de criação.

## 22. Blacklist

O ACS deverá possuir blacklist independente do cadastro de roteadores.

Tipos:

```text
SERIAL_NUMBER
MAC_ADDRESS
OUI
IP_ADDRESS
PRODUCT_CLASS
```

Campos:

- ID;
- tipo;
- valor;
- motivo;
- ativo;
- expiração;
- usuário responsável;
- data de criação;
- data de atualização.

## 23. Sessões ACS

Cada comunicação relevante poderá ser associada a uma sessão.

Dados sugeridos:

- ID;
- roteador;
- início;
- fim;
- endereço IP;
- protocolo;
- status;
- eventos informados;
- User-Agent;
- metadata.

## 24. Parâmetros do roteador

O sistema deverá suportar parâmetros variáveis por fabricante, modelo e versão.

Exemplos:

```text
Device.DeviceInfo.SoftwareVersion
Device.DeviceInfo.UpTime
Device.WiFi.SSID.1.SSID
InternetGatewayDevice.DeviceInfo.SoftwareVersion
```

Tabela:

```text
acs.router_parameters
```

Campos:

- roteador;
- parâmetro;
- valor;
- tipo;
- writable;
- data de coleta.

## 25. Eventos do roteador

Eventos que devem poder ser armazenados:

- BOOT;
- BOOTSTRAP;
- PERIODIC;
- CONNECTION REQUEST;
- VALUE CHANGE;
- TRANSFER COMPLETE.

Dados mínimos:

- roteador;
- tipo;
- timestamp;
- sessão;
- payload;
- metadata.

## 26. Configurações

Configurações globais devem permanecer em:

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

## 27. Auditoria

Ações importantes devem gerar auditoria em:

```text
public.audit_logs
```

Exemplos:

- cadastro de roteador;
- alteração de modelo;
- alteração de firmware;
- upload de firmware;
- criação de comando;
- cancelamento de comando;
- bloqueio de equipamento;
- remoção da blacklist;
- alteração de configurações.

## 28. Requisitos funcionais

- **RF-001:** cadastrar, editar, visualizar e desativar fabricantes.
- **RF-002:** cadastrar modelos de roteadores e suas capacidades.
- **RF-003:** associar múltiplas versões de hardware a um modelo.
- **RF-004:** cadastrar firmware associado a modelo e hardware.
- **RF-005:** permitir cadastrar firmware sem arquivo e anexá-lo depois.
- **RF-006:** cadastrar roteadores por nome, modelo, serial e MAC.
- **RF-007:** registrar a última comunicação recebida do roteador.
- **RF-008:** calcular estado de conectividade pela última comunicação.
- **RF-009:** armazenar e alterar configurações Wi-Fi.
- **RF-010:** criar comandos remotos.
- **RF-011:** processar todo comando por fila assíncrona.
- **RF-012:** executar retentativas segundo política configurada.
- **RF-013:** preservar histórico de tentativas.
- **RF-014:** preservar histórico de mudança de status.
- **RF-015:** permitir expiração de comandos.
- **RF-016:** permitir cancelamento de comandos não concluídos.
- **RF-017:** bloquear equipamentos via blacklist.
- **RF-018:** solicitar atualização de firmware compatível.
- **RF-019:** ativar ou desativar monitoramento.
- **RF-020:** ativar ou desativar anti-furto.

## 29. Regras de negócio

- **RN-001:** número de série deve ser único quando fornecido.
- **RN-002:** MAC principal deve ser único quando fornecido.
- **RN-003:** firmware só pode ser enviado para equipamentos compatíveis.
- **RN-004:** não pode existir mais de uma atualização de firmware ativa por roteador.
- **RN-005:** comando concluído não pode retornar para estado ativo.
- **RN-006:** comando cancelado exige novo comando para nova execução.
- **RN-007:** comando expirado não pode ser executado.
- **RN-008:** toda tentativa deve estar associada a um comando.
- **RN-009:** contador de tentativas deve refletir tentativas registradas.
- **RN-010:** senha Wi-Fi não pode ser armazenada em texto puro.
- **RN-011:** blacklist deve ser verificada durante a identificação do equipamento.
- **RN-012:** alterações administrativas relevantes devem possuir responsável.

## 30. Requisitos não funcionais

- **RNF-001:** PostgreSQL como banco de dados.
- **RNF-002:** UUID para entidades principais.
- **RNF-003:** datas usando `TIMESTAMPTZ`.
- **RNF-004:** JSONB para payloads variáveis.
- **RNF-005:** credenciais e secrets não devem ficar em texto puro.
- **RNF-006:** operações sensíveis devem ser auditáveis.
- **RNF-007:** fila deve impedir processamento concorrente do mesmo comando.
- **RNF-008:** suporte a idempotência.
- **RNF-009:** suporte a múltiplos workers.
- **RNF-010:** logs estruturados e métricas para serviços e workers.

## 31. Processamento da fila

A busca de comandos disponíveis deve considerar:

```text
status IN (PENDING, WAITING_DEVICE, RETRY_SCHEDULED)
available_at <= now()
expires_at > now()
```

Ordenação:

```text
priority DESC
available_at ASC
created_at ASC
```

O worker deve utilizar lock no PostgreSQL para prevenir execução duplicada, por exemplo:

```sql
SELECT ... FOR UPDATE SKIP LOCKED;
```

## 32. Fluxo — ativação de roteador

```text
1. Operador abre o roteador.
2. Seleciona "Ativar".
3. NetControl altera desired_status para ACTIVE.
4. Sistema cria ACTIVATE.
5. Comando entra como PENDING.
6. Worker verifica se pode executar.
7. Se depender do CPE, muda para WAITING_DEVICE.
8. Roteador realiza novo Inform.
9. ACS identifica comando pendente.
10. Status muda para PROCESSING.
11. Tentativa é registrada.
12. Parâmetros são enviados.
13. Equipamento confirma execução.
14. Tentativa = SUCCESS.
15. Comando = COMPLETED.
16. reported_status = ACTIVE.
```

## 33. Fluxo — falha de comando

```text
PROCESSING
↓
Tentativa = FAILED
↓
Verifica attempts_count < max_attempts
↓
Sim: RETRY_SCHEDULED
Não: FAILED
```

## 34. Fluxo — atualização de firmware

```text
Operador seleciona firmware
↓
Sistema valida modelo
↓
Sistema valida hardware
↓
Sistema valida arquivo
↓
Cria UPDATE_FIRMWARE
↓
Comando entra na fila
↓
Roteador recebe solicitação
↓
Download do firmware
↓
Transfer Complete
↓
ACS valida resultado
↓
COMPLETED ou FAILED
```

## 35. Interface do módulo ACS

Menu inicial:

```text
Dashboard
Roteadores
Modelos
Firmwares
Fila de comandos
Blacklist
Configurações
```

## 36. Tela de roteadores

A listagem deve apresentar:

- nome;
- modelo;
- MAC;
- serial;
- status;
- firmware;
- endereço IP;
- última conexão.

Filtros:

- nome;
- serial;
- MAC;
- modelo;
- fabricante;
- status;
- firmware.

Ações rápidas:

- reiniciar;
- ativar;
- bloquear;
- atualizar firmware;
- ativar anti-furto;
- desativar anti-furto;
- ativar monitoramento;
- desativar monitoramento.

## 37. Tela do roteador

Sugestão de abas:

```text
Visão geral
Wi-Fi
Informações do dispositivo
Parâmetros
Comandos
Eventos
Sessões
Histórico
```

A visão geral deve exibir:

- modelo;
- serial;
- MAC;
- IP;
- firmware;
- hardware;
- status desejado;
- status reportado;
- última conexão;
- último Inform;
- primeira conexão.

## 38. Tela de fila de comandos

Cada comando deve exibir:

- comando;
- roteador;
- status;
- tentativas;
- máximo de tentativas;
- última tentativa;
- próxima tentativa;
- prioridade;
- data de criação;
- expiração;
- último erro.

Ao abrir o comando deve ser possível consultar:

- ciclo de vida completo;
- tentativas;
- requests;
- responses;
- erros;
- usuário responsável.

## 39. API

Estrutura sugerida:

```text
/api/v1/acs/routers
/api/v1/acs/router-models
/api/v1/acs/firmwares
/api/v1/acs/commands
/api/v1/acs/blacklist
```

Exemplos:

```text
POST /api/v1/acs/routers/{id}/commands
GET  /api/v1/acs/routers/{id}/commands
GET  /api/v1/acs/commands/{id}
POST /api/v1/acs/commands/{id}/cancel
```

## 40. Critérios de sucesso do MVP

O MVP será considerado funcional quando permitir:

1. cadastrar fabricantes;
2. cadastrar modelos;
3. cadastrar versões de hardware;
4. cadastrar firmwares;
5. realizar upload de firmware;
6. cadastrar roteadores;
7. registrar comunicação dos roteadores;
8. visualizar última conexão;
9. consultar informações básicas do equipamento;
10. registrar Wi-Fi 2.4 e 5 GHz;
11. criar comandos;
12. processar comandos de forma assíncrona;
13. realizar retentativas;
14. registrar histórico de tentativas;
15. registrar ciclo de vida dos comandos;
16. ativar roteador;
17. bloquear roteador;
18. reiniciar roteador;
19. atualizar firmware;
20. ativar/desativar anti-furto;
21. ativar/desativar monitoramento;
22. administrar blacklist;
23. visualizar dashboard operacional.

## 41. Fora do escopo inicial

- billing;
- financeiro;
- CRM;
- cadastro completo de clientes;
- provisionamento de OLT;
- gerenciamento de ONU/ONT fora do fluxo ACS;
- topology discovery;
- inventário completo de rede;
- mapas geográficos;
- configuração automática de switches;
- aplicação mobile.

## 42. Expansões futuras

Possíveis módulos futuros:

```text
NetControl
├── Core / public
├── ACS
├── Inventory
├── Monitoring
├── Provisioning
├── Network
└── Audit
```

Possíveis evoluções do ACS:

- TR-369 / USP;
- configuração em massa;
- campanhas de firmware;
- rollout gradual;
- grupos de roteadores;
- templates de configuração;
- provisionamento zero-touch;
- dashboards por fabricante;
- análise de qualidade Wi-Fi;
- coleta de clientes conectados;
- métricas de sinal;
- métricas WAN;
- diagnóstico remoto;
- comparação entre configuração desejada e reportada.

## 43. Resumo da arquitetura de dados

```text
public
├── users
├── roles
├── permissions
├── user_roles
├── role_permissions
├── system_settings
└── audit_logs

acs
├── manufacturers
├── router_models
├── router_model_hardware_versions
├── firmwares
├── routers
├── router_wifi_radios
├── router_sessions
├── router_commands
├── router_command_attempts
├── router_command_events
├── router_blacklist
├── router_parameters
└── router_events
```

## 44. Princípios de implementação

- separar o núcleo do NetControl dos módulos;
- não acoplar o ACS a fabricantes específicos;
- manter compatibilidade com parâmetros variáveis;
- preservar histórico de operações;
- tornar comandos idempotentes;
- permitir processamento distribuído da fila;
- distinguir estado desejado de estado reportado;
- utilizar a última comunicação como referência de conectividade;
- tratar o ACS como sistema assíncrono;
- permitir crescimento sem grandes alterações no modelo de dados.

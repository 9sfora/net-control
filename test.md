# Teste manual da API com um TP-Link

Este roteiro parte do ponto em que o TP-Link já enviou um `Inform` para o ACS e aparece na tabela de roteadores desconhecidos.

Execute os comandos no computador onde o Docker Compose está rodando. Eles usam a API administrativa em `http://localhost:8080`. O roteador continua usando `http://187.19.219.19:23080/cwmp` como ACS.

Requisitos locais: `curl` e `jq`.

> Atenção: aplique primeiro em um TP-Link de teste. Trocar SSID, senha, canal ou DHCP pode derrubar sua conexão com o roteador. Se estiver conectado por Wi-Fi, prefira executar o teste por cabo.

## 1. Preparar as variáveis

```bash
# Endereço local da API administrativa.
export API_URL='http://localhost:8080'

# Host configurado como APPLICATION_FQDN no ambiente local.
export API_HOST='localhost'

# Tenant criado pelo bootstrap.
export TENANT_SLUG='netcontrol'

# Usuário administrativo criado pelo bootstrap.
export ADMIN_EMAIL='admin@netcontrol.local'

# Informe a senha real do administrador sem gravá-la no histórico do shell.
read -rsp 'Senha do administrador: ' ADMIN_PASSWORD; echo
```

## 2. Autenticar

```bash
# Faz login e guarda a resposta completa para inspeção.
curl -sS -X POST "$API_URL/api/v1/auth/login" -H "Host: $API_HOST" -H 'Content-Type: application/json' --data "$(jq -n --arg tenant "$TENANT_SLUG" --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{tenant:$tenant,email:$email,password:$password}')" | tee /tmp/netcontrol-login.json | jq

# Extrai o token JWT que será usado nos próximos comandos.
export TOKEN="$(jq -r '.access_token' /tmp/netcontrol-login.json)"

# Confirma a identidade autenticada e as permissões disponíveis.
curl -sS "$API_URL/api/v1/auth/me" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | jq
```

O login deve retornar `access_token`. Se `TOKEN` ficar vazio ou `null`, pare aqui e confira tenant, e-mail e senha.

## 3. Localizar o TP-Link desconhecido

```bash
# Lista os roteadores que enviaram Inform e ainda não foram configurados.
curl -sS "$API_URL/api/v1/acs/unknown-routers?limit=100&offset=0" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | tee /tmp/netcontrol-unknown.json | jq

# Mostra uma lista compacta para você identificar o equipamento pelo serial, OUI e Product Class.
jq -r '.data[] | [.id,.manufacturer,.oui,.product_class,.serial_number,.hardware_version] | @tsv' /tmp/netcontrol-unknown.json

# Informe o número de série exatamente como apareceu na listagem anterior.
read -rp 'Serial do TP-Link: ' ROUTER_SERIAL

# Extrai o UUID do roteador desconhecido escolhido.
export UNKNOWN_ROUTER_ID="$(jq -r --arg serial "$ROUTER_SERIAL" '.data[] | select(.serial_number==$serial) | .id' /tmp/netcontrol-unknown.json | head -n1)"

# Extrai o OUI reportado pelo TP-Link para criar um modelo compatível.
export ROUTER_OUI="$(jq -r --arg serial "$ROUTER_SERIAL" '.data[] | select(.serial_number==$serial) | .oui' /tmp/netcontrol-unknown.json | head -n1)"

# Extrai o Product Class reportado pelo TP-Link para criar um modelo compatível.
export ROUTER_PRODUCT_CLASS="$(jq -r --arg serial "$ROUTER_SERIAL" '.data[] | select(.serial_number==$serial) | .product_class' /tmp/netcontrol-unknown.json | head -n1)"

# Extrai a versão de hardware reportada; ela será apenas informativa neste primeiro teste.
export ROUTER_HARDWARE="$(jq -r --arg serial "$ROUTER_SERIAL" '.data[] | select(.serial_number==$serial) | .hardware_version // empty' /tmp/netcontrol-unknown.json | head -n1)"

# Exibe os valores selecionados antes de alterar qualquer cadastro.
printf 'UNKNOWN_ROUTER_ID=%s\nOUI=%s\nPRODUCT_CLASS=%s\nHARDWARE=%s\n' "$UNKNOWN_ROUTER_ID" "$ROUTER_OUI" "$ROUTER_PRODUCT_CLASS" "$ROUTER_HARDWARE"
```

`UNKNOWN_ROUTER_ID` não pode ficar vazio. O modelo criado abaixo precisa ter o mesmo OUI e Product Class enviados pelo equipamento.

## 4. Criar o fabricante TP-Link

Se o fabricante já estiver cadastrado, pule o `POST` e use a seção de busca logo abaixo.

```bash
# Cadastra a TP-Link no catálogo do tenant.
curl -sS -X POST "$API_URL/api/v1/acs/manufacturers" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' --data '{"name":"TP-Link","description":"Equipamentos TP-Link usados no teste CWMP"}' | tee /tmp/netcontrol-manufacturer.json | jq

# Caso o POST tenha retornado conflito, localiza o fabricante TP-Link já existente.
curl -sS "$API_URL/api/v1/acs/manufacturers?search=TP-Link" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | tee /tmp/netcontrol-manufacturers.json | jq

# Usa o UUID criado pelo POST ou, se já existia, o primeiro resultado chamado TP-Link.
export MANUFACTURER_ID="$(jq -r '.data.id // empty' /tmp/netcontrol-manufacturer.json 2>/dev/null)"; [ -n "$MANUFACTURER_ID" ] || export MANUFACTURER_ID="$(jq -r '.data[] | select((.name|ascii_downcase)=="tp-link") | .id' /tmp/netcontrol-manufacturers.json | head -n1)"

# Confirma o UUID do fabricante selecionado.
printf 'MANUFACTURER_ID=%s\n' "$MANUFACTURER_ID"
```

## 5. Criar o modelo compatível

Troque `TP-Link de teste` pelo modelo comercial real se desejar. OUI e Product Class são preenchidos automaticamente com os valores informados pelo CPE.

```bash
# Cria o modelo TR-069 usando exatamente a identidade reportada pelo roteador.
curl -sS -X POST "$API_URL/api/v1/acs/router-models" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' --data "$(jq -n --arg manufacturer_id "$MANUFACTURER_ID" --arg oui "$ROUTER_OUI" --arg product_class "$ROUTER_PRODUCT_CLASS" '{manufacturer_id:$manufacturer_id,name:("TP-Link "+$product_class),manufacturer_model_name:$product_class,supports_wifi_2_4:true,supports_wifi_5:true,supported_auth_modes:["WPA2_PSK"],product_class:$product_class,oui:$oui,protocol:"TR-069",description:"Modelo criado no teste manual"}')" | tee /tmp/netcontrol-model.json | jq

# Extrai o UUID do modelo recém-criado.
export ROUTER_MODEL_ID="$(jq -r '.data.id' /tmp/netcontrol-model.json)"

# Consulta o modelo e confere OUI, Product Class e capacidades cadastradas.
curl -sS "$API_URL/api/v1/acs/router-models/$ROUTER_MODEL_ID" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | jq
```

Se receber `ACS_CATALOG_CONFLICT`, o mesmo modelo já existe. Liste os modelos e defina manualmente o UUID correto:

```bash
# Lista modelos filtrados pelo fabricante para localizar um cadastro existente.
curl -sS "$API_URL/api/v1/acs/router-models?manufacturer_id=$MANUFACTURER_ID" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | jq

# Preencha este valor somente se precisar reutilizar um modelo existente.
export ROUTER_MODEL_ID='COLE-AQUI-O-UUID-DO-MODELO'
```

## 6. Promover o roteador desconhecido

Neste primeiro teste não vinculamos uma versão de hardware do catálogo. O backend continuará preservando a versão reportada pelo TP-Link.

```bash
# Move o equipamento da tabela de desconhecidos para a tabela de roteadores configurados.
curl -sS -X POST "$API_URL/api/v1/acs/unknown-routers/$UNKNOWN_ROUTER_ID/configure" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' --data "$(jq -n --arg name "TP-Link $ROUTER_SERIAL" --arg model "$ROUTER_MODEL_ID" '{name:$name,router_model_id:$model}')" | tee /tmp/netcontrol-router.json | jq

# Extrai o UUID definitivo do roteador configurado.
export ROUTER_ID="$(jq -r '.data.id' /tmp/netcontrol-router.json)"

# Consulta o detalhe do roteador configurado.
curl -sS "$API_URL/api/v1/acs/routers/$ROUTER_ID" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | jq
```

## 7. Salvar Wi-Fi, canal, DHCP e DNS desejados

O exemplo configura:

- SSID 2,4 GHz `NetControl-24` no canal 6;
- SSID 5 GHz `NetControl-5G` no canal 36;
- senha WPA2 igual nas duas bandas;
- DHCP entre `192.168.1.100` e `192.168.1.200`;
- concessão DHCP de 24 horas;
- DNS `1.1.1.1` e `1.0.0.1`.

Antes de executar, ajuste a rede para corresponder ao endereço LAN atual do TP-Link. A API ainda não altera o IP LAN do roteador; portanto, não use uma faixa que pertença a outra sub-rede.

```bash
# Solicita a nova senha Wi-Fi sem gravá-la no histórico do shell.
read -rsp 'Nova senha Wi-Fi: ' WIFI_PASSWORD; echo

# Salva toda a configuração desejada; esta chamada ainda não envia nada ao roteador.
curl -sS -X PUT "$API_URL/api/v1/acs/routers/$ROUTER_ID/configuration" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' --data "$(jq -n --arg password "$WIFI_PASSWORD" '{wifi:[{band:"2.4GHZ",ssid:"NetControl-24",password:$password,security_mode:"WPA2_PSK",channel_mode:"MANUAL",channel:6,bandwidth_mhz:20,enabled:true},{band:"5GHZ",ssid:"NetControl-5G",password:$password,security_mode:"WPA2_PSK",channel_mode:"MANUAL",channel:36,bandwidth_mhz:80,enabled:true}],network:{dhcp_enabled:true,dhcp_start:"192.168.1.100",dhcp_end:"192.168.1.200",dhcp_lease_seconds:86400,dns_primary:"1.1.1.1",dns_secondary:"1.0.0.1"}}')" | tee /tmp/netcontrol-configuration.json | jq

# Consulta o roteador e confirma a configuração salva; a senha aparece apenas como password_configured.
curl -sS "$API_URL/api/v1/acs/routers/$ROUTER_ID" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | jq '.data | {id,name,configuration_version,wifi,network}'
```

Se o TP-Link possuir somente 2,4 GHz, use este payload alternativo:

```bash
# Salva uma configuração compatível com equipamento de banda única 2,4 GHz.
curl -sS -X PUT "$API_URL/api/v1/acs/routers/$ROUTER_ID/configuration" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' --data "$(jq -n --arg password "$WIFI_PASSWORD" '{wifi:[{band:"2.4GHZ",ssid:"NetControl-24",password:$password,security_mode:"WPA2_PSK",channel_mode:"MANUAL",channel:6,bandwidth_mhz:20,enabled:true}],network:{dhcp_enabled:true,dhcp_start:"192.168.1.100",dhcp_end:"192.168.1.200",dhcp_lease_seconds:86400,dns_primary:"1.1.1.1",dns_secondary:"1.0.0.1"}}')" | tee /tmp/netcontrol-configuration.json | jq
```

## 8. Criar o comando de ativação

```bash
# Cria um ACTIVATE idempotente contendo o snapshot da configuração desejada.
curl -sS -X POST "$API_URL/api/v1/acs/routers/$ROUTER_ID/commands" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -H "Idempotency-Key: tplink-$ROUTER_ID-config-1" --data '{"type":"ACTIVATE"}' | tee /tmp/netcontrol-command.json | jq

# Extrai o UUID do comando para acompanhar sua execução.
export COMMAND_ID="$(jq -r '.data.id' /tmp/netcontrol-command.json)"

# Consulta o comando, suas tentativas e seu histórico de eventos.
curl -sS "$API_URL/api/v1/acs/commands/$COMMAND_ID" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | jq
```

O worker normalmente move o comando de `PENDING` para `WAITING_DEVICE`. O backend só o entrega quando o TP-Link iniciar uma sessão CWMP, geralmente no próximo `Periodic Inform` ou após reiniciar o equipamento.

## 9. Acompanhar a execução

```bash
# Mostra o estado atual do comando a cada dois segundos; encerre com Ctrl+C.
while true; do curl -sS "$API_URL/api/v1/acs/commands/$COMMAND_ID" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | jq -r '[.data.id,.data.type,.data.status,.data.attempts_count,.data.error_code,.data.error_message] | @tsv'; sleep 2; done
```

Em outro terminal:

```bash
# Acompanha os Inform, a entrega de SetParameterValues e eventuais falhas CWMP.
docker compose logs -f backend worker
```

Estados esperados:

```text
PENDING → WAITING_DEVICE → PROCESSING → COMPLETED
```

Se terminar em `FAILED`, consulte o detalhe completo:

```bash
# Exibe o FaultCode, FaultString, tentativas e eventos registrados para diagnóstico.
curl -sS "$API_URL/api/v1/acs/commands/$COMMAND_ID" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | jq
```

## 10. Verificar o resultado

Depois de `COMPLETED`, conecte novamente ao Wi-Fi usando o novo SSID e senha. Em seguida, confirme no painel do TP-Link:

- canal de cada rádio;
- faixa e tempo de concessão do DHCP;
- DNS distribuído aos clientes;
- dispositivos recebendo endereços dentro da nova faixa.

Também consulte novamente o backend:

```bash
# Confirma estado reportado, último Inform, sessões recentes e comandos recentes do roteador.
curl -sS "$API_URL/api/v1/acs/routers/$ROUTER_ID" -H "Host: $API_HOST" -H "Authorization: Bearer $TOKEN" | jq
```

## UPnP

UPnP ainda **não está implementado** nesta versão da API. O campo consta no projeto funcional, mas ainda falta adicioná-lo ao banco, ao payload de configuração e ao mapeamento CWMP específico do equipamento. Incluir `upnp_enabled` no JSON atual resultará em `400 VALIDATION_ERROR`, pois a API rejeita campos desconhecidos.

Além disso, os caminhos TR-069 usados para Wi-Fi, DHCP, DNS e UPnP podem variar conforme o modelo e firmware TP-Link. Atualmente o backend envia o perfil genérico TR-181 (`Device.*`). Se o comando retornar Fault `9005`, `9007` ou outro erro de parâmetro, guarde a resposta do passo 9: ela será usada para criar o profile específico desse modelo.

## Observação sobre o teste por HTTP

O TP-Link está usando:

```text
http://187.19.219.19:23080/cwmp
```

O cadastro pelo primeiro `Inform` funciona em HTTP, como você já confirmou. Para esse teste, configure `ACS_COOKIE_SECURE=false` no `.env` e recrie o backend. Assim o TP-Link pode devolver o cookie `ACS_SESSION` na continuação da sessão HTTP.

Para produção, publique `https://acs.seudominio.com/cwmp` com certificado válido e mantenha `ACS_COOKIE_SECURE=true`, que é o valor padrão.

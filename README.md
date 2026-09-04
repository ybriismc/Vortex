# Vortex

Proxy de Minecraft: Bedrock Edition escrito em Go, construído sobre o
[Spectrum](https://github.com/cooldogedev/spectrum).

O Vortex empacota o Spectrum em um binário pronto para produção: configuração em
YAML, balanceamento de servidores, filtro de pacotes (rate limit, bloqueio e
limite de tamanho), animações de transferência, resource packs e o serviço de API
para os servidores downstream.

---

## Sobre o Spectrum

O Spectrum é a base do Vortex. Os pontos que importam para quem for operar este proxy:

- **Protocolo próprio, sem RakNet entre proxy e servidor.** A comunicação com os
  servidores downstream usa [Spectral](https://github.com/cooldogedev/spectral),
  QUIC ou TCP no lugar do RakNet e do protocolo padrão do Minecraft, o que traz
  mais confiabilidade e desempenho. Cada servidor mantém **uma única conexão**, e
  cada jogador ocupa uma *stream* dentro dela — dialing mais barato e menos
  overhead de conexão.
- **Repasse de pacotes sem decodificação.** Por padrão o proxy não decodifica os
  pacotes do jogador: ele apenas repassa os bytes. Só os IDs listados em
  `client_decode` (no Vortex: `security.decode_packets`) são decodificados. É
  isso que mantém o throughput alto e a latência baixa.
- **Discovery.** Em vez de uma lista fixa de servidores, o Spectrum pergunta a uma
  interface `Discovery` para onde mandar o jogador no login, e para onde mandá-lo
  em caso de queda (fallback). Como a chamada é assíncrona, ela pode fazer
  operações bloqueantes — consulta a banco de dados, HTTP — e também funciona como
  balanceador de carga.
- **Processor.** A interface `Processor` intercepta cada pacote que entra e sai da
  sessão, além de eventos como transferência, fallback, cache e desconexão.
  Qualquer pacote pode ser cancelado. É o lugar certo para anti-cheat, filtros e
  telemetria.
- **Stateless.** O proxy não guarda registro dos servidores existentes.
  Transferir um jogador é apenas mandar o pacote de transfer do Spectrum a partir
  do servidor downstream, o que torna a escala horizontal simples.
- **Determinístico.** O Spectrum não traduz entidades: ele confia nos
  identificadores determinísticos fornecidos pelos servidores downstream, evitando
  toda uma camada de tradução (e de bugs).
- **Serviço de API.** Um serviço TCP separado permite que os servidores
  transfiram e expulsem jogadores, com autenticação por segredo e registro de
  pacotes e handlers próprios.
- **Animações de transferência.** A troca de servidor pode ser mascarada por
  animações de câmera (`dimension`, `fade`, `smooth`, `ease`).
- **Implementações compatíveis** do lado do servidor:
  [spectrum-df](https://github.com/cooldogedev/spectrum-df) (Dragonfly) e
  [spectrum-pm](https://github.com/cooldogedev/spectrum-pm) (PocketMine-MP).

> O servidor por trás do proxy **precisa** falar o protocolo do Spectrum. Um
> servidor Bedrock comum (RakNet puro) não funciona como downstream.

---

## O que o Vortex adiciona

| Recurso | Descrição |
| --- | --- |
| Configuração em YAML | `config.yml` gerado automaticamente na primeira execução |
| Balanceamento | Pools de servidores primários e de fallback com `round_robin`, `random` ou `first` |
| Guard | `Processor` com rate limit por sessão, bloqueio de pacotes por ID e limite de tamanho |
| Login controlado | O guard e a animação são anexados **antes** do login da sessão começar |
| Animações | Seleção por configuração; câmera acompanha o jogador nos modos `smooth` e `ease` |
| API | Serviço TCP do Spectrum com autenticação por segredo, ligado por configuração |
| Resource packs | Carregamento de uma pasta, com suporte a chaves de conteúdo |
| Operação | Logs em texto ou JSON e desligamento limpo em `SIGINT`/`SIGTERM` |

---

## Requisitos

- Go 1.25 ou superior
- Um servidor downstream com Spectrum (spectrum-df ou spectrum-pm)

## Instalação

```bash
git clone https://github.com/ybriismc/Vortex.git
cd Vortex
make build
./vortex
```

Na primeira execução o `config.yml` é criado com os valores padrão. Ajuste-o e
suba o proxy de novo. Para usar outro caminho:

```bash
./vortex -config /etc/vortex/config.yml
```

---

## Configuração

O arquivo comentado está em [`config.example.yml`](config.example.yml). Resumo das seções:

### `proxy`

| Chave | Padrão | Descrição |
| --- | --- | --- |
| `addr` | `:19132` | Endereço UDP onde os jogadores entram |
| `name` / `sub_name` | `Vortex Proxy` / `Vortex` | Texto exibido na lista de servidores |
| `transport` | `spectral` | Transporte até os servidores: `spectral` ou `quic` |
| `xbox_authentication` | `true` | Exige conta Xbox Live autenticada |
| `max_players` | `0` | Limite de jogadores (`0` = ilimitado) |
| `latency_interval` | `3000` | Intervalo em ms do relatório de latência |
| `login_timeout` | `60` | Tempo limite em segundos da sequência de login |
| `shutdown_message` | `Vortex closed.` | Mensagem enviada no desligamento |
| `sync_protocol` | `false` | Fala com o servidor na versão de protocolo do cliente |
| `transfer_animation` | `dimension` | `none`, `dimension`, `fade`, `smooth` ou `ease` |

### `servers`

`primary` recebe os jogadores no login; `fallback` é usado quando o servidor
atual cai no meio do jogo. `balancer` escolhe o endereço: `round_robin`,
`random` ou `first`.

### `security`

- `rate_limit`: pacotes por segundo por sessão; ao estourar, o Vortex descarta os
  pacotes (`drop`) ou desconecta o jogador (`kick`).
- `blocked_packets`: IDs de pacotes do cliente que nunca chegam ao servidor. O ID
  é lido direto do cabeçalho, **sem decodificar o pacote**.
- `decode_packets`: IDs que o proxy decodifica por completo (equivale ao
  `client_decode` do Spectrum). Quanto menor a lista, mais rápido o proxy.
- `max_packet_size`: tamanho máximo, em bytes, de um pacote do cliente.

### `api`

Serviço TCP para os servidores downstream. Com `secret` preenchido, o servidor
precisa enviar o mesmo segredo no `ConnectionRequest`. Pacotes disponíveis:

| ID | Pacote | Efeito |
| --- | --- | --- |
| `0` | `ConnectionRequest` | Autentica o servidor no serviço |
| `1` | `ConnectionResponse` | Resposta da autenticação |
| `2` | `Kick` | Desconecta um jogador pelo nome |
| `3` | `Transfer` | Transfere um jogador para outro endereço |

Do lado do jogo, os pacotes do protocolo Spectrum (IDs a partir de `500`)
permitem que o servidor peça `Transfer`, `Flush`, `Latency` e `UpdateCache`
diretamente na conexão da sessão.

---

## Estrutura do projeto

```
cmd/vortex          binário e leitura de flags
internal/config     configuração em YAML, padrões e validação
internal/discovery  server.Discovery com pools e balanceamento
internal/guard      session.Processor com rate limit e filtros
internal/proxy      montagem do Spectrum, animações, packs e API
```

---

## Ajuste de kernel (Linux)

Sob carga alta, os buffers de rede padrão do Linux podem ser pequenos demais e
causar erros ou desconexões aleatórias. Recomendação do Spectrum:

```bash
sysctl -w net.core.rmem_max=7500000
sysctl -w net.core.wmem_max=7500000
sysctl -w net.ipv4.tcp_rmem="4096 87380 7500000"
```

---

## Créditos

- [Spectrum](https://github.com/cooldogedev/spectrum) e
  [Spectral](https://github.com/cooldogedev/spectral), por
  [cooldogedev](https://github.com/cooldogedev)
- [gophertunnel](https://github.com/sandertv/gophertunnel), por
  [Sandertv](https://github.com/Sandertv)

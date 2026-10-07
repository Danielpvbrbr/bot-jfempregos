# Radar de Vagas JF & Região (WhatsApp Bot)

Robô agregador e monitor de vagas de emprego em tempo real desenvolvido em **Go (Golang)**. O sistema realiza web scraping periódico nos principais portais de empregos de Juiz de Fora e região (**JF Empregos**, **InfoJobs** e **Indeed**), processa novos anúncios e envia notificações automáticas formatadas diretamente para um grupo do WhatsApp via **whatsmeow**.

---

## Funcionalidades

- ⚡ **Monitoramento em Tempo Real:** Varredura periódica configurada a cada 2 minutos.
- 🔄 **Scraping Concorrente e Paralelo:** Execução simultânea dos scrapers via Go Goroutines e `sync.WaitGroup`, reduzindo latência.
- 🖼️ **Disparo com Imagem e Legenda:** Vagas do JF Empregos incluem o logotipo/foto da empresa anexado com texto formatado na legenda.
- 🛡️ **Bypass Antibot no Indeed:** Headers HTTP customizados para simular navegação desktop legítima.
- 🧹 **Deduplicação Inteligente em Memória:** Controle concorrente seguro com `sync.RWMutex` para evitar reenvio de vagas já notificadas.
- 📱 **Sessão Persistente no WhatsApp:** Armazenamento local da sessão e chaves criptográficas em banco SQLite (`modernc.org/sqlite`), evitando ter que escanear QR Code a cada reinicialização.
- ⏱️ **Rate Limiting Natural:** Intervalo seguro entre disparos para respeitar os limites de envio da Meta.

---

## Fontes Monitoradas

| Portal | Modalidade de Captura | Particularidades |
| :--- | :--- | :--- |
| **JF Empregos** | HTML Parsing (`goquery`) | Extração completa de cargo, empresa, escolaridade, vagas e **logotipo da empresa** |
| **InfoJobs** | HTML Parsing (`goquery`) | Ordenação por publicação recente (`order=date`), extração de salário e higienização de tooltips |
| **Indeed** | HTML Parsing (`goquery`) | Identificação por `data-jk`, contorno de desafio de requisições e link canônico |

---

## Estrutura do Projeto

```text
├── models/
│   └── vaga.go           # Estrutura unificada de dados (Vaga)
├── scrapers/
│   ├── jfempregos.go     # Scraper do portal JF Empregos
│   ├── infojobs.go       # Scraper do portal InfoJobs
│   └── indeed.go         # Scraper do portal Indeed
├── whatsapp/
│   └── client.go         # Conexão whatsmeow, autenticação QR Code e envio de mídia/texto
├── go.mod
├── go.sum
├── .gitignore            # Ignora bancos de sessão SQLite (*.db)
├── main.go               # Ponto de entrada, agendador concorrente e orquestrador
└── README.md
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite" // SQLite puro em Go sem necessidade de CGO/GCC
)

// Atualize para o JID correto capturado no seu terminal
const grupoWhatsAppJID = "120363430923383824@g.us"

type Vaga struct {
	ID           string
	Cargo        string
	Empresa      string
	Nivel        string
	SubArea      string
	NumeroVagas  string
	Escolaridade string
	Cidade       string
	URL          string
}

var (
	vagasVistas = make(map[string]bool)
	muVagas     sync.RWMutex
	waClient    *whatsmeow.Client
)

// Inicia a sessão e ligação nativa do WhatsApp
func iniciarWhatsApp() (*whatsmeow.Client, error) {
	ctx := context.Background()

	dbLog := waLog.Stdout("Database", "ERROR", true)

	// Parâmetros WAL e busy_timeout evitam o erro SQLITE_BUSY / database is locked
	dbURI := "file:whatsapp_sessao.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)"
	container, err := sqlstore.New(ctx, "sqlite", dbURI, dbLog)
	if err != nil {
		return nil, err
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, err
	}

	clientLog := waLog.Stdout("Client", "ERROR", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	// Interceta mensagens recebidas para mostrar o JID no terminal
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			if v.Info.IsGroup {
				//fmt.Printf("\n📌 [WhatsApp] Mensagem detetada no grupo! JID: %s\n", v.Info.Chat.String())
			}
		}
	})

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(ctx)
		err = client.Connect()
		if err != nil {
			return nil, err
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("\n📱 Abre o WhatsApp no telemóvel e digitaliza o código QR abaixo:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				fmt.Printf("Evento de autenticação: %s\n", evt.Event)
			}
		}
	} else {
		err = client.Connect()
		if err != nil {
			return nil, err
		}
		fmt.Println("✅ WhatsApp autenticado e ligado com sucesso!")
	}

	return client, nil
}

// Envia a vaga formatada para o grupo do WhatsApp
func enviarWhatsApp(v Vaga) error {
	if waClient == nil || !waClient.IsConnected() {
		return fmt.Errorf("cliente do WhatsApp desconectado")
	}

	if strings.TrimSpace(grupoWhatsAppJID) == "" {
		return fmt.Errorf("JID do grupo não configurado na constante 'grupoWhatsAppJID'")
	}

	var sb strings.Builder
	sb.WriteString("🚨 *NOVA VAGA NO JF EMPREGOS!*\n\n")
	sb.WriteString(fmt.Sprintf("📌 *Cargo:* %s (%s)\n", v.Cargo, v.Nivel))
	sb.WriteString(fmt.Sprintf("🏢 *Empresa:* %s\n", v.Empresa))

	if v.SubArea != "" {
		sb.WriteString(fmt.Sprintf("🏷️ *Área:* %s\n", v.SubArea))
	}
	if v.NumeroVagas != "" {
		sb.WriteString(fmt.Sprintf("👥 *Vagas:* %s\n", v.NumeroVagas))
	}
	if v.Escolaridade != "" {
		sb.WriteString(fmt.Sprintf("🎓 *Escolaridade:* %s\n", v.Escolaridade))
	}
	if v.Cidade != "" {
		sb.WriteString(fmt.Sprintf("📍 *Cidade:* %s\n", v.Cidade))
	}
	sb.WriteString(fmt.Sprintf("\n🔗 *Link:* %s", v.URL))

	jid, err := types.ParseJID(grupoWhatsAppJID)
	if err != nil {
		return fmt.Errorf("JID inválido: %w", err)
	}

	msg := &waProto.Message{
		Conversation: proto.String(sb.String()),
	}

	_, err = waClient.SendMessage(context.Background(), jid, msg)
	return err
}

func buscarVagas() ([]Vaga, error) {
	endpoint := "https://www.jfempregos.com.br/getitens-vag?tipo=listagem&pag=1&maximo=20&busca=&order=&niveis_vaga=&ocupacoes=&niveis_escolaridade=&cidades=&deficientes=&modalidade="
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status inesperado: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var vagas []Vaga

	doc.Find(".boxTabelas").Each(func(i int, box *goquery.Selection) {
		linkTag := box.Find("a[href*='vaga/']").First()
		rawHref, exists := linkTag.Attr("href")
		if !exists {
			return
		}

		partes := strings.Split(rawHref, "vaga/")
		if len(partes) < 2 {
			return
		}
		id := strings.TrimSpace(partes[1])

		urlCompleta := "https://www.jfempregos.com.br/vaga/" + id
		cargo := strings.TrimSpace(box.Find("h3").First().Text())
		empresa := strings.TrimSpace(box.Find(".nome_empresa").First().Text())
		nivel := strings.TrimSpace(box.Find(".tipo").First().Text())
		subArea := strings.TrimSpace(box.Find("h3 + p").First().Text())

		vaga := Vaga{
			ID:      id,
			Cargo:   cargo,
			Empresa: empresa,
			Nivel:   nivel,
			SubArea: subArea,
			URL:     urlCompleta,
		}

		box.Find("table tr td").Each(func(j int, td *goquery.Selection) {
			bText := strings.TrimSpace(td.Find("b").Text())
			pText := strings.TrimSpace(td.Find("p").Text())

			if strings.Contains(bText, "Número de Vagas") {
				vaga.NumeroVagas = pText
			} else if strings.Contains(bText, "Escolaridade") {
				vaga.Escolaridade = pText
			} else if strings.Contains(bText, "Cidade") {
				vaga.Cidade = pText
			}
		})

		if vaga.ID != "" && vaga.Cargo != "" {
			vagas = append(vagas, vaga)
		}
	})

	return vagas, nil
}

func monitorar(primeiraExecucao bool) {
	fmt.Printf("[%s] A procurar novas vagas no JF Empregos...\n", time.Now().Format("15:04:05"))

	vagas, err := buscarVagas()
	if err != nil {
		log.Printf("Erro ao procurar vagas: %v\n", err)
		return
	}

	for _, v := range vagas {
		muVagas.Lock()
		jaVista := vagasVistas[v.ID]
		if !jaVista {
			vagasVistas[v.ID] = true
		}
		muVagas.Unlock()

		if !jaVista {
			if primeiraExecucao {
				continue
			}

			fmt.Println("==================================================")
			fmt.Printf("🚨 NOVA VAGA DETETADA: [%s] %s\n", v.ID, v.Cargo)
			fmt.Println("==================================================")

			if err := enviarWhatsApp(v); err != nil {
				log.Printf("Falha ao enviar para o WhatsApp: %v\n", err)
			} else {
				fmt.Println("✅ Notificação enviada para o grupo do WhatsApp!")
			}

			time.Sleep(2 * time.Second)
		}
	}
}

func main() {
	fmt.Println("🚀 A inicializar rastreador nativo em Go...")

	var err error
	waClient, err = iniciarWhatsApp()
	if err != nil {
		log.Fatalf("Falha crítica ao iniciar WhatsApp: %v", err)
	}

	// 1. Carrega as vagas que já existem na memória para NÃO enviar spam ao iniciar
	fmt.Println("📋 A carregar vagas atuais em memória...")
	monitorar(true)

	// 2. Inicia o monitoramento contínuo
	fmt.Println("👀 Bot ativo! A monitorizar novas vagas a cada 2 minutos...")
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	// 3. Captura Ctrl+C para desligamento seguro
	parada := make(chan os.Signal, 1)
	signal.Notify(parada, os.Interrupt, syscall.SIGTERM)

	go func() {
		for range ticker.C {
			monitorar(false) // Aqui ele enviará apenas se surgir um ID novo
		}
	}()

	<-parada
	fmt.Println("\nA terminar ligação com o WhatsApp...")
	waClient.Disconnect()
}

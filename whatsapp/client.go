package whatsapp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"

	"capitarVagas/models"
)

type Service struct {
	Client    *whatsmeow.Client
	Container *sqlstore.Container
	GrupoJID  string
	mu        sync.Mutex
}

func NovoService(grupoJID string) (*Service, error) {
	ctx := context.Background()
	dbLog := waLog.Stdout("Database", "ERROR", true)

	dbURI := "file:whatsapp_sessao.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)"
	container, err := sqlstore.New(ctx, "sqlite", dbURI, dbLog)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir banco de dados: %w", err)
	}

	s := &Service{
		Container: container,
		GrupoJID:  grupoJID,
	}

	if err := s.conectarOuAutenticar(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Service) conectarOuAutenticar(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	deviceStore, err := s.Container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("erro ao recuperar dispositivo: %w", err)
	}

	clientLog := waLog.Stdout("Client", "ERROR", true)
	s.Client = whatsmeow.NewClient(deviceStore, clientLog)

	s.Client.AddEventHandler(func(evt interface{}) {
		switch evt.(type) {
		case *events.LoggedOut:
			fmt.Println("\n⚠️ Sessão encerrada no WhatsApp (desconectado). A resetar sessão e gerar novo QR Code...")
			go s.reconectarComNovoQR()

		case *events.Disconnected:
			fmt.Println("\n⚠️ Conexão perdida. O whatsmeow tentará reconectar em segundo plano...")

		case *events.Connected:
			fmt.Println("✅ Conexão ativa com os servidores do WhatsApp!")
		}
	})

	if s.Client.Store.ID == nil {
		// Sem credenciais salvas: gera o QR code
		qrChan, _ := s.Client.GetQRChannel(ctx)
		if err := s.Client.Connect(); err != nil {
			return err
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("\n📱 WhatsApp desconectado. Leia o QR Code abaixo com o telemóvel:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			}
		}
	} else {
		if err := s.Client.Connect(); err != nil {
			return err
		}
		fmt.Println("✅ WhatsApp autenticado com credenciais em cache!")
	}

	return nil
}

func (s *Service) reconectarComNovoQR() {
	s.mu.Lock()
	if s.Client != nil {
		s.Client.Disconnect()
	}
	s.mu.Unlock()

	time.Sleep(2 * time.Second)

	ctx := context.Background()
	if s.Client != nil && s.Client.Store != nil {
		_ = s.Client.Store.Delete(ctx)
	}

	if err := s.conectarOuAutenticar(ctx); err != nil {
		fmt.Printf("❌ Falha na reautenticação com QR: %v\n", err)
	}
}

// Baixa os bytes da imagem da vaga
func baixarImagem(url string) ([]byte, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status HTTP ao baixar imagem: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	return data, mimeType, nil
}

func (s *Service) EnviarVaga(v models.Vaga) error {
	s.mu.Lock()
	cli := s.Client
	s.mu.Unlock()

	if cli == nil || !cli.IsConnected() {
		return fmt.Errorf("WhatsApp desconectado no momento")
	}

	// 1. Monta o texto / legenda no Estilo 2 (Direto e com destaque)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📢 *VAGA ABERTA [%s]*\n\n", strings.ToUpper(v.Origem)))

	// Cargo em destaque e maiúsculas
	sb.WriteString(fmt.Sprintf("🔹 *CARGO:* %s", strings.ToUpper(v.Cargo)))
	if v.Nivel != "" {
		sb.WriteString(fmt.Sprintf(" (%s)", v.Nivel))
	}
	sb.WriteString("\n")

	if v.Empresa != "" {
		sb.WriteString(fmt.Sprintf("🔹 *EMPRESA:* %s\n", v.Empresa))
	}

	// Agrupa Vagas e Escolaridade de forma limpa se existirem
	detalhes := []string{}
	if v.NumeroVagas != "" {
		detalhes = append(detalhes, v.NumeroVagas)
	}
	if v.Escolaridade != "" {
		detalhes = append(detalhes, v.Escolaridade)
	}
	if len(detalhes) > 0 {
		sb.WriteString(fmt.Sprintf("🔹 *DETALHES:* %s\n", strings.Join(detalhes, " | ")))
	}

	if v.Cidade != "" {
		sb.WriteString(fmt.Sprintf("🔹 *LOCAL:* %s\n", v.Cidade))
	}

	sb.WriteString(fmt.Sprintf("\n🔗 *Link da Vaga:*\n%s\n\n", v.URL))
	sb.WriteString("👥 *Compartilhe com quem precisa!*\n")
	sb.WriteString("👉 *Participe do nosso grupo de vagas de JF e região:*\n")
	sb.WriteString("https://chat.whatsapp.com/EZn5H2TImjK1u6iFJaq9K6")

	jid, err := types.ParseJID(s.GrupoJID)
	if err != nil {
		return fmt.Errorf("JID inválido: %w", err)
	}

	ctx := context.Background()

	// 2. Se houver imagem, envia com a legenda acoplada e miniatura embutida
	if v.ImagemURL != "" {
		imgBytes, mimeType, errImg := baixarImagem(v.ImagemURL)
		if errImg == nil && len(imgBytes) > 0 {
			uploadResp, errUpload := cli.Upload(ctx, imgBytes, whatsmeow.MediaImage)
			if errUpload == nil {
				msgImg := &waProto.Message{
					ImageMessage: &waProto.ImageMessage{
						Caption:       proto.String(sb.String()),
						Mimetype:      proto.String(mimeType),
						URL:           &uploadResp.URL,
						DirectPath:    &uploadResp.DirectPath,
						MediaKey:      uploadResp.MediaKey,
						FileEncSHA256: uploadResp.FileEncSHA256,
						FileSHA256:    uploadResp.FileSHA256,
						FileLength:    proto.Uint64(uint64(len(imgBytes))),
						JPEGThumbnail: imgBytes,
					},
				}
				_, err = cli.SendMessage(ctx, jid, msgImg)
				return err
			}
		}
	}

	// 3. Fallback: Se não tiver imagem ou o upload falhar, envia como mensagem de texto
	msgTexto := &waProto.Message{
		Conversation: proto.String(sb.String()),
	}
	_, err = cli.SendMessage(ctx, jid, msgTexto)
	return err
}

func (s *Service) Encerrar() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Client != nil {
		s.Client.Disconnect()
	}
}

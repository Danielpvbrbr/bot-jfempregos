package whatsapp

import (
	"context"
	"fmt"
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
		switch v := evt.(type) {
		case *events.LoggedOut:
			fmt.Println("\n⚠️ Sessão encerrada no WhatsApp (desconectado). A resetar sessão e gerar novo QR Code...")
			go s.reconectarComNovoQR()

		case *events.Disconnected:
			_ = v // ou simplesmente não declare 'v' se mudar o switch
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
	// Remove o dispositivo invalidado para exigir login limpo
	if s.Client != nil && s.Client.Store != nil {
		_ = s.Client.Store.Delete(ctx)
	}

	if err := s.conectarOuAutenticar(ctx); err != nil {
		fmt.Printf("❌ Falha na reautenticação com QR: %v\n", err)
	}
}

func (s *Service) EnviarVaga(v models.Vaga) error {
	s.mu.Lock()
	cli := s.Client
	s.mu.Unlock()

	if cli == nil || !cli.IsConnected() {
		return fmt.Errorf("WhatsApp desconectado no momento")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🚨 *NOVA VAGA [%s]!*\n\n", strings.ToUpper(v.Origem)))
	sb.WriteString(fmt.Sprintf("📌 *Cargo:* %s", v.Cargo))
	if v.Nivel != "" {
		sb.WriteString(fmt.Sprintf(" (%s)", v.Nivel))
	}
	sb.WriteString("\n")

	if v.Empresa != "" {
		sb.WriteString(fmt.Sprintf("🏢 *Empresa:* %s\n", v.Empresa))
	}
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

	jid, err := types.ParseJID(s.GrupoJID)
	if err != nil {
		return fmt.Errorf("JID inválido: %w", err)
	}

	msg := &waProto.Message{
		Conversation: proto.String(sb.String()),
	}

	_, err = cli.SendMessage(context.Background(), jid, msg)
	return err
}

func (s *Service) Encerrar() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Client != nil {
		s.Client.Disconnect()
	}
}

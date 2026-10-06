package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"capitarVagas/models"
	"capitarVagas/scrapers"
	"capitarVagas/whatsapp"
)

const grupoWhatsAppJID = "120363430923383824@g.us"

var (
	vagasVistas = make(map[string]bool)
	muVagas     sync.RWMutex
	waService   *whatsapp.Service
)

func processarVagas(vagas []models.Vaga, primeiraExecucao bool) {
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
			fmt.Printf("🚨 NOVA VAGA [%s]: %s\n", v.Origem, v.Cargo)
			fmt.Println("==================================================")

			if err := waService.EnviarVaga(v); err != nil {
				log.Printf("Falha no envio WhatsApp: %v\n", err)
			} else {
				fmt.Println("✅ Notificação enviada para o WhatsApp!")
			}

			// Pausa de 3 segundos para manter o canal com comportamento natural
			time.Sleep(3 * time.Second)
		}
	}
}

func monitorar(primeiraExecucao bool) {
	fmt.Printf("[%s] A verificar novas vagas em todos os scrapers...\n", time.Now().Format("15:04:05"))

	var wg sync.WaitGroup

	// 1. Scraping JF Empregos em paralelo
	wg.Add(1)
	go func() {
		defer wg.Done()
		vagasJF, err := scrapers.BuscarJFEmpregos()
		if err != nil {
			log.Printf("Erro [JF Empregos]: %v\n", err)
			return
		}
		processarVagas(vagasJF, primeiraExecucao)
	}()

	// 2. Scraping InfoJobs em paralelo
	wg.Add(1)
	go func() {
		defer wg.Done()
		vagasInfo, err := scrapers.BuscarInfoJobs()
		if err != nil {
			log.Printf("Erro [InfoJobs]: %v\n", err)
			return
		}
		processarVagas(vagasInfo, primeiraExecucao)
	}()

	wg.Wait()
}

func main() {
	fmt.Println("🚀 A inicializar agregador de vagas...")

	var err error
	waService, err = whatsapp.NovoService(grupoWhatsAppJID)
	if err != nil {
		log.Fatalf("Falha crítica ao iniciar WhatsApp: %v", err)
	}

	fmt.Println("📋 A carregar histórico inicial em memória...")
	monitorar(true)

	fmt.Println("👀 Agregador pronto! Próximas checagens a cada 2 minutos.")
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	parada := make(chan os.Signal, 1)
	signal.Notify(parada, os.Interrupt, syscall.SIGTERM)

	go func() {
		for range ticker.C {
			monitorar(false)
		}
	}()

	<-parada
	fmt.Println("\nA terminar aplicação...")
	waService.Encerrar()
}

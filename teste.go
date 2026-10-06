package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type VagaIndeed struct {
	ID         string
	Cargo      string
	Empresa    string
	Cidade     string
	Salario    string
	Modalidade string
	URL        string
}

func main() {
	targetURL := "https://br.indeed.com/jobs?q=&l=Juiz+de+Fora%2C+MG&sort=date"
	fmt.Printf("🔍 Consultando Indeed: %s\n\n", targetURL)

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		fmt.Printf("Erro na requisição: %v\n", err)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="128", "Not;A=Brand";v="24", "Google Chrome";v="128"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Erro no Do: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Status inesperado: %d\n", resp.StatusCode)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Printf("Erro goquery: %v\n", err)
		return
	}

	var vagas []VagaIndeed
	vistas := make(map[string]bool)

	// O container raiz de cada anúncio de vaga no Indeed
	doc.Find("div.cardOutline, .job_seen_beacon").Each(func(i int, s *goquery.Selection) {
		// 1. Captura o job key (ID único)
		jk, _ := s.Find("a[data-jk]").Attr("data-jk")
		if jk == "" {
			jk, _ = s.Attr("data-jk")
		}
		if jk == "" || vistas[jk] {
			return
		}

		// 2. Cargo
		cargo := strings.TrimSpace(s.Find("h2.jobTitle span, a[data-jk] span").First().Text())
		if cargo == "" {
			return
		}

		// 3. Empresa
		empresa := strings.TrimSpace(s.Find("[data-testid='company-name']").First().Text())
		if empresa == "" {
			empresa = strings.TrimSpace(s.Find(".companyName").First().Text())
		}
		if empresa == "" {
			empresa = "Confidencial / Não informada"
		}

		// 4. Localização / Cidade
		cidade := strings.TrimSpace(s.Find("[data-testid='text-location']").First().Text())
		if cidade == "" {
			cidade = "Juiz de Fora - MG"
		}

		// 5. Salário ou Metadados (Metadata container)
		salario := "A combinar"
		salarioTag := s.Find("[data-testid='attribute_snippet_testid'], .salary-snippet-container, .metadata.salary-snippet-container").First()
		if salarioTag.Length() > 0 && strings.TrimSpace(salarioTag.Text()) != "" {
			salario = strings.TrimSpace(salarioTag.Text())
		}

		// 6. Link canônico direto para a vaga
		linkCompleto := fmt.Sprintf("https://br.indeed.com/viewjob?jk=%s", jk)

		vistas[jk] = true
		vagas = append(vagas, VagaIndeed{
			ID:      jk,
			Cargo:   cargo,
			Empresa: empresa,
			Cidade:  cidade,
			Salario: salario,
			URL:     linkCompleto,
		})
	})

	fmt.Printf("✅ Total de vagas únicas capturadas: %d\n\n", len(vagas))

	limite := 5
	if len(vagas) < limite {
		limite = len(vagas)
	}

	for i := 0; i < limite; i++ {
		v := vagas[i]
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("📌 [%d] %s (ID: %s)\n", i+1, v.Cargo, v.ID)
		fmt.Printf("🏢 Empresa: %s\n", v.Empresa)
		fmt.Printf("📍 Cidade:  %s\n", v.Cidade)
		fmt.Printf("💰 Salário: %s\n", v.Salario)
		fmt.Printf("🔗 Link:    %s\n", v.URL)
	}
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
}

package scrapers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"capitarVagas/models"

	"github.com/PuerkitoBio/goquery"
)

// URL com busca em Juiz de Fora ordenada pelas vagas mais recentes (&sort=date)
const indeedJFURL = "https://br.indeed.com/jobs?q=&l=Juiz+de+Fora%2C+MG&sort=date"

func BuscarIndeed() ([]models.Vaga, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", indeedJFURL, nil)
	if err != nil {
		return nil, err
	}

	// Headers essenciais para simular navegador real e contornar checagens antibot
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
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status inesperado Indeed: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var vagas []models.Vaga
	vistas := make(map[string]bool)

	// Percorre os cartões de vagas
	doc.Find("div.cardOutline, .job_seen_beacon").Each(func(i int, s *goquery.Selection) {
		// 1. Identificador único do Indeed (jk)
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
		empresa = strings.Join(strings.Fields(empresa), " ")

		// 4. Cidade / Localização
		cidade := strings.TrimSpace(s.Find("[data-testid='text-location']").First().Text())
		if cidade == "" {
			cidade = "Juiz de Fora - MG"
		}

		// 5. Salário ou Benefícios
		salario := "A combinar"
		salarioTag := s.Find("[data-testid='attribute_snippet_testid'], .salary-snippet-container, .metadata.salary-snippet-container").First()
		if salarioTag.Length() > 0 && strings.TrimSpace(salarioTag.Text()) != "" {
			salario = strings.TrimSpace(salarioTag.Text())
		}

		// 6. Link direto
		linkCompleto := fmt.Sprintf("https://br.indeed.com/viewjob?jk=%s", jk)

		vistas[jk] = true
		vagas = append(vagas, models.Vaga{
			ID:      "indeed_" + jk,
			Origem:  "Indeed",
			Cargo:   cargo,
			Empresa: empresa,
			Nivel:   salario, // Mapeado para exibir o Salário no WhatsApp
			Cidade:  cidade,
			URL:     linkCompleto,
		})
	})

	return vagas, nil
}

package scrapers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"capitarVagas/models"

	"github.com/PuerkitoBio/goquery"
)

// URL com poblacion=5205417 (Juiz de Fora) ordenada por publicação recente (&order=date)
const infojobsJFURL = "https://www.infojobs.com.br/empregos.aspx?poblacion=5205417&order=date"

func BuscarInfoJobs() ([]models.Vaga, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", infojobsJFURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status inesperado retornado pelo InfoJobs: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var vagas []models.Vaga
	vistas := make(map[string]bool)

	// Percorre os cartões de vagas, ignorando o dropdown de vagas semelhantes
	doc.Find("div.js_vacancyLoad, div.grid-row div[data-id]").Each(func(i int, s *goquery.Selection) {
		if s.ParentsFiltered(".js_collapsibleGroupDiv").Length() > 0 {
			return
		}

		id, _ := s.Attr("data-id")
		if id == "" || vistas[id] {
			return
		}

		// 1. Cargo
		cargo := strings.TrimSpace(s.Find(".js_vacancyTitle").Text())
		if cargo == "" {
			cargo = strings.TrimSpace(s.Find("h2").First().Text())
		}
		if cargo == "" {
			return
		}

		// 2. Link direto
		urlRelativa, exists := s.Attr("data-href")
		if !exists || urlRelativa == "" {
			urlRelativa, _ = s.Find("a[href*='vaga-de']").First().Attr("href")
		}
		if !strings.HasPrefix(urlRelativa, "http") && urlRelativa != "" {
			urlRelativa = "https://www.infojobs.com.br" + urlRelativa
		}

		// 3. Empresa (remove tooltips e ícones)
		empresaSel := s.Find(".text-body a, .text-body span.text-nowrap, a[href*='empresa-']").First()
		empresaSel.Find("span[data-bs-toggle='tooltip'], svg").Remove()
		empresa := strings.TrimSpace(empresaSel.Text())
		if empresa == "" {
			empresa = "Confidencial / Não informada"
		}
		empresa = strings.Join(strings.Fields(empresa), " ")

		// 4. Cidade (extrai o texto e remove a tag oculta de distância)
		cidadeDiv := s.Find("div.mb-8").First().Clone()
		cidadeDiv.Find("span").Remove()
		cidade := strings.TrimSpace(cidadeDiv.Text())
		if cidade == "" {
			cidade = "Juiz de Fora - MG"
		}

		// 5. Salário, Escolaridade e Modelo
		salario := "A combinar"
		escolaridade := ""
		modelo := "Presencial"

		s.Find("div.d-inline-flex.flex-wrap > div").Each(func(j int, item *goquery.Selection) {
			txt := strings.Join(strings.Fields(item.Text()), " ")
			html, _ := item.Html()

			if strings.Contains(html, "#money") {
				salario = txt
			} else if strings.Contains(html, "#graduate-hat") {
				escolaridade = txt
			} else if strings.Contains(html, "#buildings") || strings.Contains(html, "#house-and-building") {
				modelo = txt
			}
		})

		// 6. Resumo curto da vaga
		resumo := strings.TrimSpace(s.Find("div.text-medium:not(.small):not(.d-inline-flex)").Last().Text())
		resumo = strings.Join(strings.Fields(resumo), " ")

		vistas[id] = true
		vagas = append(vagas, models.Vaga{
			ID:           "infojobs_" + id,
			Origem:       "InfoJobs",
			Cargo:        cargo,
			Empresa:      empresa,
			Nivel:        salario, // Mapeado no WhatsApp como nível/salário
			SubArea:      resumo,  // Resumo descritivo da vaga
			Escolaridade: escolaridade,
			Cidade:       fmt.Sprintf("%s (%s)", cidade, modelo), // Ex: Juiz de Fora - MG (Presencial)
			URL:          urlRelativa,
		})
	})

	return vagas, nil
}

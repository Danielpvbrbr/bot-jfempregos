package scrapers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"capitarVagas/models"

	"github.com/PuerkitoBio/goquery"
)

func BuscarJFEmpregos() ([]models.Vaga, error) {
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
		return nil, fmt.Errorf("status inesperado JF Empregos: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var vagas []models.Vaga

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

		vaga := models.Vaga{
			ID:      "jf_" + id, // Prefixo para evitar colisão com outros portais
			Origem:  "JF Empregos",
			Cargo:   strings.TrimSpace(box.Find("h3").First().Text()),
			Empresa: strings.TrimSpace(box.Find(".nome_empresa").First().Text()),
			Nivel:   strings.TrimSpace(box.Find(".tipo").First().Text()),
			SubArea: strings.TrimSpace(box.Find("h3 + p").First().Text()),
			URL:     "https://www.jfempregos.com.br/vaga/" + id,
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

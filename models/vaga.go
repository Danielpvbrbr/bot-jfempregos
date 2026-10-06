package models

type Vaga struct {
	ID           string
	Origem       string // "JF Empregos", "InfoJobs", etc.
	Cargo        string
	Empresa      string
	Nivel        string
	SubArea      string
	NumeroVagas  string
	Escolaridade string
	Cidade       string
	URL          string
}

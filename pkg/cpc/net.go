package cpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type CpcResponse struct {
	Data CpcData `json:"data"`
}

type CpcData struct {
	Records []CpcRecord `json:"data"`
	Total   int         `json:"recordsTotal"`
}

type CpcRecord struct {
	Name          string `json:"afectado"`
	Document      string `json:"identificador"`
	Administrator bool   `json:"administrador"`
	Debtor        bool   `json:"deudor"`
	Disabled      bool   `json:"inhabilitado"`
}

func SingleCPCRequest(document string) (*CpcRecord, error) {
	client := &http.Client{}
	var data = strings.NewReader(fmt.Sprintf("draw=2&columns%%5B0%%5D%%5Bdata%%5D=afectado&columns%%5B0%%5D%%5Bname%%5D=&columns%%5B0%%5D%%5Bsearchable%%5D=true&columns%%5B0%%5D%%5Borderable%%5D=false&columns%%5B0%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B0%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&columns%%5B1%%5D%%5Bdata%%5D=identificador&columns%%5B1%%5D%%5Bname%%5D=&columns%%5B1%%5D%%5Bsearchable%%5D=true&columns%%5B1%%5D%%5Borderable%%5D=false&columns%%5B1%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B1%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&columns%%5B2%%5D%%5Bdata%%5D=deudor&columns%%5B2%%5D%%5Bname%%5D=&columns%%5B2%%5D%%5Bsearchable%%5D=true&columns%%5B2%%5D%%5Borderable%%5D=false&columns%%5B2%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B2%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&columns%%5B3%%5D%%5Bdata%%5D=inhabilitado&columns%%5B3%%5D%%5Bname%%5D=&columns%%5B3%%5D%%5Bsearchable%%5D=true&columns%%5B3%%5D%%5Borderable%%5D=false&columns%%5B3%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B3%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&columns%%5B4%%5D%%5Bdata%%5D=administrador&columns%%5B4%%5D%%5Bname%%5D=&columns%%5B4%%5D%%5Bsearchable%%5D=true&columns%%5B4%%5D%%5Borderable%%5D=false&columns%%5B4%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B4%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&columns%%5B5%%5D%%5Bdata%%5D=summary&columns%%5B5%%5D%%5Bname%%5D=&columns%%5B5%%5D%%5Bsearchable%%5D=true&columns%%5B5%%5D%%5Borderable%%5D=false&columns%%5B5%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B5%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&start=0&length=10&search%%5Bvalue%%5D=&search%%5Bregex%%5D=false&_org_registradores_rpc_concursal_web_ConcursalWebPortlet_formDate=1689664386073&_org_registradores_rpc_concursal_web_ConcursalWebPortlet_section1=true&_org_registradores_rpc_concursal_web_ConcursalWebPortlet_section2=true&_org_registradores_rpc_concursal_web_ConcursalWebPortlet_section3=true&_org_registradores_rpc_concursal_web_ConcursalWebPortlet_identificador=%s&_org_registradores_rpc_concursal_web_ConcursalWebPortlet_provincia=0&_org_registradores_rpc_concursal_web_ConcursalWebPortlet_checkboxNames=section1%%2Csection2%%2Csection3", document))
	req, err := http.NewRequest("POST", "https://www.publicidadconcursal.es/consulta-publicidad-concursal-new?p_p_id=org_registradores_rpc_concursal_web_ConcursalWebPortlet&p_p_lifecycle=2&p_p_state=normal&p_p_mode=view&p_p_resource_id=%2Fafectado%2Fsearch&p_p_cacheability=cacheLevelPage", data)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", "https://www.publicidadconcursal.es")
	req.Header.Set("Referer", "https://www.publicidadconcursal.es/consulta-publicidad-concursal-new")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("sec-ch-ua", `"Not.A/Brand";v="8", "Chromium";v="114", "Google Chrome";v="114"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var cpcresponse CpcResponse
	if err = json.Unmarshal(bodyText, &cpcresponse); err != nil {
		return nil, err
	}

	if len(cpcresponse.Data.Records) < 1 {
		return nil, nil
	}

	return &cpcresponse.Data.Records[0], nil
}

func ThreadCPCRequester(streams []chan CpcCsvRow, retries int) (chan CpcRecord, chan string, chan int) {
	var wg sync.WaitGroup
	matches := make(chan CpcRecord)
	errors := make(chan string)
	updates := make(chan int)
	for i := range streams {
		wg.Add(1)
		i2 := i
		go func() {
			defer wg.Done()
			for row := range streams[i2] {
				var record *CpcRecord
				var err error
				retry := true
				b := 0

				for b <= retries && retry {
					record, err = SingleCPCRequest(row.Document)
					if record != nil {
						retry = false
					}
					b++
				}

				updates <- 1
				if err != nil {
					errors <- row.Document
					continue
				}
				if record != nil {
					matches <- *record
				}
			}
		}()
	}

	go func() {
		defer close(updates)
		defer close(matches)
		defer close(errors)
		wg.Wait()
	}()

	return matches, errors, updates
}

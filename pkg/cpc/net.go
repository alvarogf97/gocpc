package cpc

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

const cpcBaseURL = "https://www.publicidadconcursal.es/consulta-publicidad-concursal-new"
const cpcSearchURL = cpcBaseURL + "?p_p_id=org_registradores_rpc_concursal_web_ConcursalOldWebPortlet&p_p_lifecycle=2&p_p_state=normal&p_p_mode=view&p_p_resource_id=%2Fafectado%2Fsearch&p_p_cacheability=cacheLevelPage"

// cpcUserAgents is a small pool of realistic desktop browser user agents.
// Each session picks one at random so concurrent threads don't all look
// like the exact same client.
var cpcUserAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15",
}

// Anti-bot tuning: the portal issues short-lived Liferay/anti-bot cookies
// (JSESSIONID, persistencia, TS...) after the first page load. Reusing the
// same session for too long or hammering the endpoint too fast triggers a
// block (403/429 or an HTML/captcha page instead of JSON). These constants
// keep each worker's session fresh and its request pace human-like.
const (
	cpcSessionMaxAge      = 5 * time.Minute
	cpcSessionMaxRequests = 30
	cpcMinDelay           = 400 * time.Millisecond
	cpcMaxDelay           = 1100 * time.Millisecond
	cpcMaxStagger         = 2 * time.Second
	cpcBackoffBase        = 800 * time.Millisecond
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

// cpcSession wraps an http.Client bound to a single cookie jar. It tracks
// its age and usage so callers can proactively rotate it before the portal's
// anti-bot cookies expire, and can be forced to refresh on demand if a
// request comes back blocked.
type cpcSession struct {
	client       *http.Client
	userAgent    string
	createdAt    time.Time
	requestCount int
}

// newCPCClient creates an http.Client with a cookie jar. The portal relies on
// Liferay session cookies (JSESSIONID, persistencia) and bot-protection
// cookies (TS...) that are only issued when visiting the search page first,
// so every request must reuse the same client/jar for both the initial GET
// and the subsequent search POST.
func newCPCClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &http.Client{Jar: jar, Timeout: 30 * time.Second}, nil
}

// primeCPCSession visits the search page so the server issues the session
// and anti-bot cookies required by the search endpoint.
func primeCPCSession(client *http.Client, userAgent string) error {
	req, err := http.NewRequest("GET", cpcBaseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	return err
}

// newCPCSession builds a fresh cpcSession, priming it with a real page visit
// so it carries valid session/anti-bot cookies.
func newCPCSession() (*cpcSession, error) {
	client, err := newCPCClient()
	if err != nil {
		return nil, err
	}
	ua := cpcUserAgents[rand.Intn(len(cpcUserAgents))]
	if err := primeCPCSession(client, ua); err != nil {
		return nil, err
	}
	return &cpcSession{client: client, userAgent: ua, createdAt: time.Now()}, nil
}

// needsRefresh reports whether the session is old or used enough that its
// cookies are likely close to expiring or getting flagged by the portal.
func (s *cpcSession) needsRefresh() bool {
	return time.Since(s.createdAt) > cpcSessionMaxAge || s.requestCount >= cpcSessionMaxRequests
}

// refresh discards the current cookies and re-primes the session from
// scratch, simulating a new visit to the page.
func (s *cpcSession) refresh() error {
	client, err := newCPCClient()
	if err != nil {
		return err
	}
	ua := cpcUserAgents[rand.Intn(len(cpcUserAgents))]
	if err := primeCPCSession(client, ua); err != nil {
		return err
	}
	s.client = client
	s.userAgent = ua
	s.createdAt = time.Now()
	s.requestCount = 0
	return nil
}

// cpcJitterSleep waits a small random amount of time before a request so
// concurrent workers don't fire requests in perfect lockstep, which is an
// easy pattern for bot-detection to flag.
func cpcJitterSleep() {
	d := cpcMinDelay + time.Duration(rand.Int63n(int64(cpcMaxDelay-cpcMinDelay)))
	time.Sleep(d)
}

// doCPCSearch performs the search POST for a document using the given
// client/user agent. It reports blocked=true when the response looks like an
// anti-bot challenge (403/429 status, or a body that isn't the expected
// JSON), so the caller can refresh the session and retry.
func doCPCSearch(client *http.Client, userAgent, document string) (record *CpcRecord, blocked bool, err error) {
	data := strings.NewReader(fmt.Sprintf("draw=2&columns%%5B0%%5D%%5Bdata%%5D=afectado&columns%%5B0%%5D%%5Bname%%5D=&columns%%5B0%%5D%%5Bsearchable%%5D=true&columns%%5B0%%5D%%5Borderable%%5D=false&columns%%5B0%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B0%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&columns%%5B1%%5D%%5Bdata%%5D=identificador&columns%%5B1%%5D%%5Bname%%5D=&columns%%5B1%%5D%%5Bsearchable%%5D=true&columns%%5B1%%5D%%5Borderable%%5D=false&columns%%5B1%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B1%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&columns%%5B2%%5D%%5Bdata%%5D=deudor&columns%%5B2%%5D%%5Bname%%5D=&columns%%5B2%%5D%%5Bsearchable%%5D=true&columns%%5B2%%5D%%5Borderable%%5D=false&columns%%5B2%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B2%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&columns%%5B3%%5D%%5Bdata%%5D=inhabilitado&columns%%5B3%%5D%%5Bname%%5D=&columns%%5B3%%5D%%5Bsearchable%%5D=true&columns%%5B3%%5D%%5Borderable%%5D=false&columns%%5B3%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B3%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&columns%%5B4%%5D%%5Bdata%%5D=administrador&columns%%5B4%%5D%%5Bname%%5D=&columns%%5B4%%5D%%5Bsearchable%%5D=true&columns%%5B4%%5D%%5Borderable%%5D=false&columns%%5B4%%5D%%5Bsearch%%5D%%5Bvalue%%5D=&columns%%5B4%%5D%%5Bsearch%%5D%%5Bregex%%5D=false&start=0&length=10&search%%5Bvalue%%5D=&search%%5Bregex%%5D=false&_org_registradores_rpc_concursal_web_ConcursalOldWebPortlet_formDate=1689664386073&_org_registradores_rpc_concursal_web_ConcursalOldWebPortlet_edictosConcursales=true&_org_registradores_rpc_concursal_web_ConcursalOldWebPortlet_publicidadRegistroMercantil=true&_org_registradores_rpc_concursal_web_ConcursalOldWebPortlet_acuerdosExtrajudiciales=true&_org_registradores_rpc_concursal_web_ConcursalOldWebPortlet_tipoIdentificador=0&_org_registradores_rpc_concursal_web_ConcursalOldWebPortlet_identificador=%s&_org_registradores_rpc_concursal_web_ConcursalOldWebPortlet_checkboxNames=edictosConcursales%%2CpublicidadRegistroMercantil%%2CacuerdosExtrajudiciales&_org_registradores_rpc_concursal_web_ConcursalOldWebPortlet_captcha=", document))
	req, err := http.NewRequest("POST", cpcSearchURL, data)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", "https://www.publicidadconcursal.es")
	req.Header.Set("Referer", cpcBaseURL)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		io.Copy(io.Discard, resp.Body)
		return nil, true, nil
	}

	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	var cpcresponse CpcResponse
	if err := json.Unmarshal(bodyText, &cpcresponse); err != nil {
		// A non-JSON body (HTML block/captcha page) means the anti-bot
		// protection kicked in rather than a real parsing failure.
		return nil, true, nil
	}

	if len(cpcresponse.Data.Records) < 1 {
		return nil, false, nil
	}
	return &cpcresponse.Data.Records[0], false, nil
}

// requestCPC runs a search using the given session, transparently rotating
// its cookies when they're stale/overused or when the portal flags the
// request as bot-like, retrying once with a fresh session before giving up.
func requestCPC(session *cpcSession, document string) (*CpcRecord, error) {
	if session.needsRefresh() {
		if err := session.refresh(); err != nil {
			return nil, err
		}
	}

	cpcJitterSleep()
	record, blocked, err := doCPCSearch(session.client, session.userAgent, document)
	if err != nil {
		return nil, err
	}
	if blocked {
		if err := session.refresh(); err != nil {
			return nil, err
		}
		time.Sleep(cpcBackoffBase)
		cpcJitterSleep()
		record, blocked, err = doCPCSearch(session.client, session.userAgent, document)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, fmt.Errorf("cpc: request for document %q was blocked by anti-bot protection", document)
		}
	}

	session.requestCount++
	return record, nil
}

// SingleCPCRequest performs a one-off lookup for a single document. It opens
// its own short-lived session (cookies primed via a real page visit) and is
// intended for isolated/manual queries. For bulk/threaded usage prefer
// ThreadCPCRequester, which reuses one session per worker instead of priming
// new cookies on every request.
func SingleCPCRequest(document string) (*CpcRecord, error) {
	session, err := newCPCSession()
	if err != nil {
		return nil, err
	}
	return requestCPC(session, document)
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

			// Stagger worker start times so N threads don't all prime a
			// session and fire their first request at the exact same
			// instant, which is an easy signal for bot detection.
			if cpcMaxStagger > 0 {
				time.Sleep(time.Duration(rand.Int63n(int64(cpcMaxStagger))))
			}

			var session *cpcSession

			for row := range streams[i2] {
				var record *CpcRecord
				var err error
				retry := true
				b := 0

				for b <= retries && retry {
					if session == nil {
						session, err = newCPCSession()
						if err != nil {
							b++
							time.Sleep(cpcBackoffBase)
							continue
						}
					}

					record, err = requestCPC(session, row.Document)
					if record != nil {
						retry = false
					}
					if err != nil {
						// Drop the session so the next attempt starts clean
						// instead of hammering the same (possibly flagged)
						// cookies again.
						session = nil
						time.Sleep(cpcBackoffBase)
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

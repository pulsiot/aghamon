package main

import (
  "embed"
  "encoding/base64"
  "encoding/json"
  "fmt"
  "html/template"
  "io"
  "net/http"
  "os"
  "strings"
  
  "github.com/labstack/echo/v4"
  "gopkg.in/yaml.v3"
  _ "golang.org/x/crypto/x509roots/fallback"
  _ "time/tzdata"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets/*
var assetFS embed.FS

// Config represents the configuration structure
type Config struct {
  AdGuard struct {
    ServerURL string `yaml:"server_url"`
    Username  string `yaml:"username"`
    Password  string `yaml:"password"`
  } `yaml:"adguard"`
}

// Client represents a DNS client from AdGuard Home
type Client struct {
  IP       string `json:"ip"`
  Name     string `json:"name"`
  Source   string `json:"source"`
  WhoisInfo struct {
    Country string `json:"country"`
    OrgName string `json:"orgname"`
    City    string `json:"city"`
  } `json:"whois_info"`
}

// ClientsResponse represents the response from AdGuard Home API
type ClientsResponse struct {
  Clients        []Client `json:"clients"`
  AutoClients    []Client `json:"auto_clients"`
  SupportedTags  []string `json:"supported_tags"`
}

// StatsResponse represents the response from AdGuard Home stats API
type StatsResponse struct {
  TimeUnits          string              `json:"time_units"`
  TopQueriedDomains  []map[string]int    `json:"top_queried_domains"`
  TopClients         []map[string]int    `json:"top_clients"`
  TopBlockedDomains  []map[string]int    `json:"top_blocked_domains"`
  TopUpstreamsResponses []map[string]int `json:"top_upstreams_responses"`
  TopUpstreamsAvgTime []map[string]float64 `json:"top_upstreams_avg_time"`
  DNSQueries         []int               `json:"dns_queries"`
  BlockedFiltering   []int               `json:"blocked_filtering"`
  NumDNSQueries      int                 `json:"num_dns_queries"`
  NumBlockedFiltering int                `json:"num_blocked_filtering"`
  AvgProcessingTime  float64             `json:"avg_processing_time"`
}

// Template represents the template structure
type Template struct {
  templates *template.Template
}

// Render implements the echo.Renderer interface
func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
  return t.templates.ExecuteTemplate(w, name, data)
}

// loadConfig loads the configuration from config.yaml
func loadConfig() (*Config, error) {
  file, err := os.Open("config.yaml")
  if err != nil {
    return nil, err
  }
  defer file.Close()

  var config Config
  decoder := yaml.NewDecoder(file)
  if err := decoder.Decode(&config); err != nil {
    return nil, err
  }

  return &config, nil
}

// getBasicAuth returns the base64 encoded basic auth string
func getBasicAuth(username, password string) string {
  auth := username + ":" + password
  return base64.StdEncoding.EncodeToString([]byte(auth))
}

// fetchClients fetches client data from AdGuard Home API
func fetchClients(config *Config) (*ClientsResponse, error) {
  client := &http.Client{}
  
  url := fmt.Sprintf("%s/control/clients", config.AdGuard.ServerURL)
  req, err := http.NewRequest("GET", url, nil)
  if err != nil {
    return nil, err
  }

  authHeader := getBasicAuth(config.AdGuard.Username, config.AdGuard.Password)
  req.Header.Set("Authorization", "Basic "+authHeader)
  req.Header.Set("Accept", "application/json")
  req.Header.Set("Referer", config.AdGuard.ServerURL+"/")

  resp, err := client.Do(req)
  if err != nil {
    return nil, err
  }
  defer resp.Body.Close()

  body, err := io.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }

  var clientsResponse ClientsResponse
  if err := json.Unmarshal(body, &clientsResponse); err != nil {
    return nil, err
  }

  return &clientsResponse, nil
}

// fetchStats fetches stats data from AdGuard Home API
func fetchStats(config *Config) (*StatsResponse, error) {
  client := &http.Client{}
  
  url := fmt.Sprintf("%s/control/stats", config.AdGuard.ServerURL)
  req, err := http.NewRequest("GET", url, nil)
  if err != nil {
    return nil, err
  }

  authHeader := getBasicAuth(config.AdGuard.Username, config.AdGuard.Password)
  req.Header.Set("Authorization", "Basic "+authHeader)
  req.Header.Set("Accept", "application/json")
  req.Header.Set("Referer", config.AdGuard.ServerURL+"/")

  resp, err := client.Do(req)
  if err != nil {
    return nil, err
  }
  defer resp.Body.Close()

  body, err := io.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }

  var statsResponse StatsResponse
  if err := json.Unmarshal(body, &statsResponse); err != nil {
    return nil, err
  }

  return &statsResponse, nil
}

// generateHTMLTable generates an HTML table from the clients data
func generateHTMLTable(clients []Client) string {
  var sb strings.Builder
  
  sb.WriteString(`<div class="table-container"><div class="mobile-table-info">Swipe horizontally to view all columns</div><table>
    <thead>
      <tr>
        <th>IP Address</th>
        <th>Name</th>
        <th>Source</th>
        <th>Country</th>
        <th>Organization</th>
        <th>City</th>
      </tr>
    </thead>
    <tbody>`)

  for _, client := range clients {
    sb.WriteString(fmt.Sprintf(`
      <tr>
        <td>%s</td>
        <td>%s</td>
        <td>%s</td>
        <td>%s</td>
        <td>%s</td>
        <td>%s</td>
      </tr>`,
      client.IP,
      client.Name,
      client.Source,
      client.WhoisInfo.Country,
      client.WhoisInfo.OrgName,
      client.WhoisInfo.City,
    ))
  }

  sb.WriteString(`</tbody></table></div>`)
  return sb.String()
}

// generateStatsTable generates an HTML table for stats data
func generateStatsTable(title string, data []map[string]int, valueLabel string) string {
  var sb strings.Builder
  
  sb.WriteString(fmt.Sprintf(`<h3>%s</h3>`, title))
  sb.WriteString(`<div class="table-container"><div class="mobile-table-info">Swipe horizontally to view all columns</div><table>
    <thead>
      <tr>
        <th>#</th>
        <th>Name</th>
        <th style="text-align: right;">` + valueLabel + `</th>
      </tr>
    </thead>
    <tbody>`)

  for i, item := range data {
    for key, value := range item {
      sb.WriteString(fmt.Sprintf(`
        <tr>
          <td>%d</td>
          <td>%s</td>
          <td style="text-align: right;">%d</td>
        </tr>`,
        i+1,
        key,
        value,
      ))
      break // Only one key-value pair per map
    }
  }

  sb.WriteString(`</tbody></table></div>`)
  return sb.String()
}

// generateUpstreamsTable generates an HTML table for upstreams data
func generateUpstreamsTable(title string, data []map[string]float64, valueLabel string) string {
  var sb strings.Builder
  
  sb.WriteString(fmt.Sprintf(`<h3>%s</h3>`, title))
  sb.WriteString(`<div class="table-container"><div class="mobile-table-info">Swipe horizontally to view all columns</div><table>
    <thead>
      <tr>
        <th>#</th>
        <th>Upstream</th>
        <th style="text-align: right;">` + valueLabel + `</th>
      </tr>
    </thead>
    <tbody>`)

  for i, item := range data {
    for key, value := range item {
      sb.WriteString(fmt.Sprintf(`
        <tr>
          <td>%d</td>
          <td>%s</td>
          <td style="text-align: right;">%.6f</td>
        </tr>`,
        i+1,
        key,
        value,
      ))
      break // Only one key-value pair per map
    }
  }

  sb.WriteString(`</tbody></table></div>`)
  return sb.String()
}

// generateHomeContent generates the home page content
func generateHomeContent() string {
  return `<h1>Welcome to Aghamon</h1>
<p>Monitor your DNS queries, clients, and upstream performance in real-time.</p>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin-top: 30px;">
    <div style="background: #e8f4fd; padding: 20px; border-radius: 5px; text-align: center;">
        <h3>📱 Clients</h3>
        <p>View connected DNS clients and their information</p>
        <a href="/clients" style="display: inline-block; background: #3498db; color: white; padding: 10px 20px; text-decoration: none; border-radius: 3px;">View Clients</a>
    </div>
    
    <div style="background: #e8f6f3; padding: 20px; border-radius: 5px; text-align: center;">
        <h3>📊 Statistics</h3>
        <p>DNS query statistics and blocked domains</p>
        <a href="/stats" style="display: inline-block; background: #27ae60; color: white; padding: 10px 20px; text-decoration: none; border-radius: 3px;">View Stats</a>
    </div>
    
    <div style="background: #fef9e7; padding: 20px; border-radius: 5px; text-align: center;">
        <h3>🌐 Upstreams</h3>
        <p>DNS upstream performance and response times</p>
        <a href="/upstreams" style="display: inline-block; background: #f39c12; color: white; padding: 10px 20px; text-decoration: none; border-radius: 3px;">View Upstreams</a>
    </div>
</div>`
}

// generateClientsContent generates the clients page content
func generateClientsContent(totalClients int, clientsTable string) string {
  return fmt.Sprintf(`<div class="header-section">
    <h1>DNS Clients</h1>
    <p>Total clients: %d</p>
</div>
%s`, totalClients, clientsTable)
}

// generateStatsContent generates the stats page content
func generateStatsContent(timeUnits string, numDNSQueries, numBlockedFiltering int, avgProcessingTime float64, topDomainsTable, topClientsTable, topBlockedTable string) string {
  return fmt.Sprintf(`<div class="header-section">
    <h1>DNS Statistics</h1>
</div>

<div class="summary">
    <p><strong>Time Period:</strong> Last 24 %s</p>
    <p><strong>Total DNS Queries:</strong> %d</p>
    <p><strong>Total Blocked Queries:</strong> %d</p>
    <p><strong>Average Processing Time:</strong> %.6f seconds</p>
</div>

%s
%s
%s`, timeUnits, numDNSQueries, numBlockedFiltering, avgProcessingTime, topDomainsTable, topClientsTable, topBlockedTable)
}

// generateUpstreamsContent generates the upstreams page content
func generateUpstreamsContent(topUpstreamsTable, topUpstreamsTimeTable string) string {
  return fmt.Sprintf(`<div class="header-section">
    <h1>DNS Upstreams</h1>
</div>

%s
%s`, topUpstreamsTable, topUpstreamsTimeTable)
}

// serveStaticFile serves embedded static files
func serveStaticFile(c echo.Context) error {
  path := c.Param("file")
  if path == "" {
    path = "index.html"
  }
  
  // Security: Only serve files from assets directory
  if strings.Contains(path, "..") {
    return c.String(http.StatusForbidden, "Forbidden")
  }
  
  data, err := assetFS.ReadFile("assets/" + path)
  if err != nil {
    return c.String(http.StatusNotFound, "File not found")
  }
  
  // Set appropriate content type based on file extension
  contentType := "application/octet-stream"
  if strings.HasSuffix(path, ".png") {
    contentType = "image/png"
  } else if strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg") {
    contentType = "image/jpeg"
  } else if strings.HasSuffix(path, ".css") {
    contentType = "text/css"
  } else if strings.HasSuffix(path, ".js") {
    contentType = "application/javascript"
  }
  
  return c.Blob(http.StatusOK, contentType, data)
}

func main() {
  e := echo.New()
  
  // Load configuration
  config, err := loadConfig()
  if err != nil {
    e.Logger.Fatal("Failed to load config:", err)
  }

  // Parse embedded templates
  templateContent, err := templateFS.ReadFile("templates/base.html")
  if err != nil {
    e.Logger.Fatal("Failed to read embedded template:", err)
  }
  
  // Setup template renderer with embedded templates
  t := &Template{
    templates: template.Must(template.New("base.html").Parse(string(templateContent))),
  }
  e.Renderer = t

  // Serve static files from embedded assets
  e.GET("/static/:file", serveStaticFile)
  e.GET("/static/", serveStaticFile)

  e.GET("/", func(c echo.Context) error {
    return c.Render(http.StatusOK, "base.html", map[string]interface{}{
      "Title": "Aghamon",
      "Content": template.HTML(generateHomeContent()),
    })
  })

  e.GET("/clients", func(c echo.Context) error {
    // Fetch clients from AdGuard Home
    clientsResponse, err := fetchClients(config)
    if err != nil {
      return c.String(http.StatusInternalServerError, fmt.Sprintf("Error fetching clients: %v", err))
    }

    // Combine both clients and auto_clients
    var allClients []Client
    allClients = append(allClients, clientsResponse.Clients...)
    allClients = append(allClients, clientsResponse.AutoClients...)

    // Generate HTML table
    htmlTable := generateHTMLTable(allClients)

    return c.Render(http.StatusOK, "base.html", map[string]interface{}{
      "Title": "DNS Clients - Aghamon",
      "Content": template.HTML(generateClientsContent(len(allClients), htmlTable)),
    })
  })

  e.GET("/stats", func(c echo.Context) error {
    // Fetch stats from AdGuard Home
    statsResponse, err := fetchStats(config)
    if err != nil {
      return c.String(http.StatusInternalServerError, fmt.Sprintf("Error fetching stats: %v", err))
    }

    // Generate HTML tables for each section
    topDomainsTable := generateStatsTable("Top Queried Domains", statsResponse.TopQueriedDomains, "Count")
    topClientsTable := generateStatsTable("Top Clients", statsResponse.TopClients, "Count")
    topBlockedTable := generateStatsTable("Top Blocked Domains", statsResponse.TopBlockedDomains, "Count")

    return c.Render(http.StatusOK, "base.html", map[string]interface{}{
      "Title": "DNS Statistics - Aghamon",
      "Content": template.HTML(generateStatsContent(
        statsResponse.TimeUnits,
        statsResponse.NumDNSQueries,
        statsResponse.NumBlockedFiltering,
        statsResponse.AvgProcessingTime,
        topDomainsTable,
        topClientsTable,
        topBlockedTable,
      )),
    })
  })

  e.GET("/upstreams", func(c echo.Context) error {
    // Fetch stats from AdGuard Home
    statsResponse, err := fetchStats(config)
    if err != nil {
      return c.String(http.StatusInternalServerError, fmt.Sprintf("Error fetching upstreams: %v", err))
    }

    // Generate HTML tables for upstreams
    topUpstreamsTable := generateStatsTable("Top Upstreams by Response Count", statsResponse.TopUpstreamsResponses, "Count")
    topUpstreamsTimeTable := generateUpstreamsTable("Top Upstreams by Average Response Time", statsResponse.TopUpstreamsAvgTime, "Time")

    return c.Render(http.StatusOK, "base.html", map[string]interface{}{
      "Title": "DNS Upstreams - Aghamon",
      "Content": template.HTML(generateUpstreamsContent(topUpstreamsTable, topUpstreamsTimeTable)),
    })
  })

  e.Logger.Fatal(e.Start(":8080"))
}

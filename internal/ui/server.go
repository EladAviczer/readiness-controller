package ui

import (
	"html/template"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

type GateStatus struct {
	Name      string
	Target    string
	IsHealthy bool
	LastCheck string
	CheckType string
	Message   string
}

var (
	stateStore = make(map[string]GateStatus)
	mu         sync.RWMutex
)

func UpdateState(ruleName, target, checkType string, healthy bool) {
	mu.Lock()
	defer mu.Unlock()

	msg := "Gate Closed"
	if healthy {
		msg = "Gate Open"
	}

	stateStore[ruleName] = GateStatus{
		Name:      ruleName,
		Target:    target,
		CheckType: checkType,
		IsHealthy: healthy,
		LastCheck: time.Now().Format("15:04:05"),
		Message:   msg,
	}
}

const htmlTmpl = `
<!DOCTYPE html>
<html>
<head>
    <title>Readiness Controller</title>
    <meta http-equiv="refresh" content="5">
    <style>
        body { font-family: sans-serif; padding: 20px; background-color: #f4f4f4; }
        h1 { text-align: center; color: #333; }
        .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 20px; }
        .card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; }
        .status-badge { padding: 5px 10px; border-radius: 4px; font-weight: bold; color: white; }
        .green { background-color: #2ecc71; }
        .red { background-color: #e74c3c; }
        .meta { font-size: 13px; color: #666; line-height: 1.6; }
        code { background: #eee; padding: 2px 4px; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>Active Gates</h1>
    <div class="grid">
        {{range .}}
        <div class="card">
            <div class="header">
                <strong>{{.Name}}</strong>
                {{if .IsHealthy}}
                    <span class="status-badge green">HEALTHY</span>
                {{else}}
                    <span class="status-badge red">FAILING</span>
                {{end}}
            </div>
            <div class="meta">
                Target: <code>{{.Target}}</code> ({{.CheckType}})<br>
                Last Check: {{.LastCheck}}<br>
                Status: {{.Message}}
            </div>
        </div>
        {{end}}
    </div>
</body>
</html>
`

var tmpl = template.Must(template.New("webpage").Parse(htmlTmpl))

func handler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	var list []GateStatus
	for _, v := range stateStore {
		list = append(list, v)
	}
	mu.RUnlock()

	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })

	if err := tmpl.Execute(w, list); err != nil {
		log.Printf("Error rendering UI template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func Start(port string) {
	http.HandleFunc("/", handler)
	log.Printf("UI Server started on port %s", port)

	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("UI Server failed to start: %v", err)
		}
	}()
}

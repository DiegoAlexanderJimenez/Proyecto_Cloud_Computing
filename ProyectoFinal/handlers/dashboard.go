package handlers

import (
    "html/template"
    "net/http"
    "sync"
    "time"
)

// Instance representa una VM aprovisionada
type Instance struct {
    Hostname  string
    IP        string
    CreatedAt time.Time
}

// store es el estado en memoria de todas las instancias activas
var (
    store   = map[string]Instance{}
    storeMu sync.Mutex
)

var tmpl = template.Must(template.ParseFiles("templates/index.html"))

// Dashboard maneja GET /
func Dashboard(w http.ResponseWriter, r *http.Request) {
    storeMu.Lock()
    instances := make([]Instance, 0, len(store))
    for _, inst := range store {
        instances = append(instances, inst)
    }
    storeMu.Unlock()

    tmpl.Execute(w, map[string]any{
        "Instances": instances,
    })
}
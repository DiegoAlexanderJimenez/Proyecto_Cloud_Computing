package main

import (
    "httpaas/handlers"
    "net/http"
    "log"
)

func main() {
    http.HandleFunc("/", handlers.Dashboard)
    http.HandleFunc("/provision", handlers.Provision)
    http.HandleFunc("/delete", handlers.Delete)
    http.HandleFunc("/logs", handlers.Logs)
    http.HandleFunc("/zone", handlers.Zone)
    http.HandleFunc("/startns", handlers.StartNS)
    http.HandleFunc("/rename", handlers.Rename)

    log.Println("Servidor corriendo en http://localhost:8081")
    log.Fatal(http.ListenAndServe(":8081", nil))
}
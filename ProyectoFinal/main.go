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

    log.Println("Servidor corriendo en http://localhost:8081")
    log.Fatal(http.ListenAndServe(":8081", nil))
}
//go:build ignore

package main

import (
    "encoding/json"
    "fmt"
    "os"
    "ghclassroom/internal/api"
    "ghclassroom/internal/config"
)

func main() {
    cfg, err := config.LoadConfig()
    if err != nil {
        fmt.Println("config error:", err)
        os.Exit(1)
    }
    classrooms, err := api.GetClassrooms(cfg.Token)
    if err != nil {
        fmt.Println("api error:", err)
        os.Exit(1)
    }
    json.NewEncoder(os.Stdout).Encode(classrooms)
}

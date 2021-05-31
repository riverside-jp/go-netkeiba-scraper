package main

import (
    "log"

    "github.com/hashicorp/hcl/v2/hclsimple"
)

var config Config

type Config struct {
    Netkeiba NetkeibaConfig `hcl:"netkeiba,block"`
}

type NetkeibaConfig struct {
    DatabaseURL string `hcl:"db_url"`
    Email       string `hcl:"email"`
    Password    string `hcl:"password"`
}

func init() {
    if err := hclsimple.DecodeFile("config.hcl", nil, &config); err != nil {
        log.Fatalf("Failed to load configuration: %s", err)
    }
}

func main() {
    // ...
}

package main

import (
    "log"
    "os"

    "github.com/hashicorp/hcl/v2/hclsimple"
    "github.com/urfave/cli/v2"
)

const (
    filenameRaceList = "race_list.txt"
)

var config Config

type Config struct {
    Netkeiba NetkeibaConfig `hcl:"netkeiba,block"`
    Path     PathConfig     `hcl:"path,block"`
}

type NetkeibaConfig struct {
    DatabaseURL string `hcl:"db_url"`
    LoginURL    string `hcl:"login_url"`
    Email       string `hcl:"email"`
    Password    string `hcl:"password"`
}

type PathConfig struct {
    DataDir string `hcl:"data_dir"`
}

func init() {
    if err := hclsimple.DecodeFile("config.hcl", nil, &config); err != nil {
        log.Fatalf("Failed to load configuration: %s", err)
    }
}

func main() {
    app := &cli.App{
        Commands: []*cli.Command{
            {
                Name: "collect",
                Usage: "Collect URL of past races from netkeiba.com",
                Flags: []cli.Flag{
                    &cli.IntFlag{
                        Name: "years",
                        Aliases: []string{"y"},
                        Usage: "Number of years to trace back in the data",
                    },
                },
                Action: cmdCollect,
            },
            {
                Name: "dump",
                Usage: "Dump past races data from netkeiba.com",
                Action: cmdDump,
            },
        },
    }

    err := app.Run(os.Args)
    if err != nil {
        log.Fatal(err)
    }
}
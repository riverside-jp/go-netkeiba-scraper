package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

func cmdDump(c *cli.Context) error {
	file, err := os.Open(filepath.Join(config.Path.DataDir, filenameRaceList))
	if err != nil {
		return xerrors.Errorf("Failed to open file: %+w", err)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return xerrors.Errorf("Failed to read file: %+w", err)
	}

	if err := loginToNetkeibaCom(config.Netkeiba.LoginURL, config.Netkeiba.Email, config.Netkeiba.Password); err != nil {
		return xerrors.Errorf("Failed to login netkeiba.com: %+w", err)
	}

	racePages := strings.Split(string(b), "\n")

	for i := 0; i < len(racePages); i++ {
		if err := dumpRacePageAsHTMLFile(config.Path.DataDir, racePages[i]); err != nil {
			log.Printf("Failed to dump %s: %+v", racePages[i], err)
			continue
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}

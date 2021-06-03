package main

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
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
		return err
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

func loginToNetkeibaCom(loginURL string, id string, password string) error {
	log.Println("Trying to login to netkeiba.com, login_id is " + id)

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return xerrors.Errorf("Failed to create cookie jar: %+v", err)
	}

	http.DefaultClient.Jar = jar

	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	values := url.Values{}
	values.Set("login_id", id)
	values.Set("pswd", password)
	values.Set("pid", "login")
	values.Set("action", "auth")

	req, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(values.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	log.Println("Sending request to " + loginURL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf("Failed to send request to %s: %+w", req.URL, err)
	}

	if resp.StatusCode != http.StatusFound {
		return xerrors.Errorf("Failed to login to netkeiba.com: %d", resp.StatusCode)
	}

	log.Println("Succeeded to login")

	return nil
}

func dumpRacePageAsHTMLFile(dumpDir string, url string) error {
	log.Println("Sending request to " + url)

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return xerrors.Errorf("Failed to send request to %s: %+v", url, err)
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return xerrors.Errorf("Failed to request to %s, status code is %d", err)
	}

	// EUC-JP -> UTF-8
	r := transform.NewReader(resp.Body, japanese.EUCJP.NewDecoder())

	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatal(err)
	}

	filename := filepath.Join(dumpDir, determineDumpHTMLFilenameFromURL(url))

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0666))
	if err != nil {
		return xerrors.Errorf("Failed to open file: %+w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(string(bytes)); err != nil {
		return xerrors.Errorf("Failed to dump HTML: %+v", err)
	}

	log.Printf("Dumped %s to %s", url, filename)

	return nil
}

func determineDumpHTMLFilenameFromURL(url string) string {
	// url looks like "https://db.netkeiba.com/race/202105020305/"
	s := strings.Split(strings.TrimRight(url, "/"), "/")

	return s[len(s)-1] + ".html"
}

package wlog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"
)

var ErrMissingSecretKey = errors.New("missing wlog secret key")

var DefaultClient = Client{}

func SendAsText(path string, data []byte) error {
	return DefaultClient.SendAsText(path, data)
}

func SendAsJSON(path string, data []byte) error {
	return DefaultClient.SendAsJSON(path, data)
}

func Send(path, mime string, data []byte) error {
	return DefaultClient.Send(path, mime, data)
}

type Client struct {
	Client    *http.Client
	SecretKey string
	URL       string
	Attempts  int
}

func (c *Client) SendAsText(path string, data []byte) error {
	tail := []byte("\n")
	return c.send(path, "text/plain", data, tail)
}

func (c *Client) SendAsJSON(path string, data []byte) error {
	tail := []byte("\n")
	return c.send(path, "application/json", data, tail)
}

func (c *Client) Send(path, mime string, data []byte) error {
	return c.send(path, mime, data, nil)
}

func (c *Client) send(path, mime string, data, tail []byte) (err error) {
	dt := 1

	n := c.Attempts
	if n == 0 {
		n = 8
	}

	for i := 0; i < n; i++ {
		r := io.Reader(bytes.NewReader(data))
		if tail != nil {
			r = io.MultiReader(r, bytes.NewReader(tail))
		}

		err = c.write(path, mime, r)
		if err == nil {
			break
		}

		wait := dt * int(time.Second)
		time.Sleep(time.Duration(rand.Intn(wait)))
		dt += dt
	}

	return
}

func (c *Client) write(path, mime string, r io.Reader) (err error) {
	key := c.SecretKey
	if key == "" {
		key = os.Getenv("WLOG_KEY")
		if key == "" {
			err = ErrMissingSecretKey
			return
		}
	}

	u := &url.URL{Scheme: "https", Host: "wlog.cloud"}
	if c.URL != "" {
		u, err = url.Parse(c.URL)
		if err != nil {
			return
		}
	}

	u.Path = path

	req, err := http.NewRequest("POST", u.String(), r)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", "go wlog/0.1 "+runtime.Version())
	req.Header.Set("Content-Type", mime)
	req.Header.Set("Authorization", c.SecretKey)

	client := http.DefaultClient
	if c.Client != nil {
		client = c.Client
	}

	rep, err := client.Do(req)
	if err != nil {
		return
	}

	result, err := ioutil.ReadAll(rep.Body)
	rep.Body.Close()

	if err != nil {
		return
	}

	if rep.StatusCode != 200 {
		err = fmt.Errorf("wlog: %s", string(result))
		return
	}

	return
}

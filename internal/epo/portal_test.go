package epo

import (
	"errors"
	"reflect"
	"testing"
)

func TestOpenUploadPageOnlyOpensOfficialURL(t *testing.T) {
	opened := ""
	browser := ""
	err := openUploadPage("chrome", func(rawURL, selectedBrowser string) error {
		opened = rawURL
		browser = selectedBrowser
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened != "https://adisspr.mfcr.cz/dpr/adis/idpr_epo/epo2/uvod/nacteni_souboru.faces" {
		t.Fatalf("otevřeno neočekávané URL: %s", opened)
	}
	if browser != "chrome" {
		t.Fatalf("opener dostal prohlížeč %q, chci chrome", browser)
	}
}

func TestBrowserCommandUsesSystemDefaultWhenMissingOrDefault(t *testing.T) {
	for _, browser := range []string{"", "default"} {
		command, err := browserCommand("darwin", browser, uploadPageURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"open", uploadPageURL}
		if !reflect.DeepEqual(command.Args, want) {
			t.Fatalf("browser=%q: args=%v, chci %v", browser, command.Args, want)
		}
	}
}

func TestBrowserCommandSelectsChromeOnMacOS(t *testing.T) {
	command, err := browserCommand("darwin", "chrome", uploadPageURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"open", "-b", "com.google.Chrome", uploadPageURL}
	if !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("args=%v, chci %v", command.Args, want)
	}
}

func TestBrowserCommandFindsLinuxExecutable(t *testing.T) {
	command, err := browserCommand("linux", "chrome", uploadPageURL, func(name string) (string, error) {
		if name == "google-chrome-stable" {
			return "/opt/google/chrome", nil
		}
		return "", errors.New("nenalezeno")
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/opt/google/chrome", uploadPageURL}
	if !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("args=%v, chci %v", command.Args, want)
	}
}

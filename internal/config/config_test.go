package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default configuration is invalid: %v", err)
	}
}

func TestLoadWritesTheDefaultConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if _, err := Load(path); err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	conf, err := Load(path)
	if err != nil {
		t.Fatalf("failed to reload the written file: %v", err)
	}

	if conf.Proxy.Addr != Default().Proxy.Addr {
		t.Fatalf("reloaded addr is %q, expected %q", conf.Proxy.Addr, Default().Proxy.Addr)
	}
}

func TestValidateRejectsAnUnknownTransport(t *testing.T) {
	conf := Default()
	conf.Proxy.Transport = "raknet"
	if err := conf.Validate(); err == nil {
		t.Fatal("expected an error for an unknown transport")
	}
}

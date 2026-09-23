package config

import "testing"

func TestFromEnv(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("missing JWT_SECRET should be an error")
	}

	t.Setenv("JWT_SECRET", "s")
	t.Setenv("PORT", "")
	t.Setenv("DB_PATH", "")
	c, err := FromEnv()
	if err != nil || c.Port != 8080 || c.DBPath != "db/nytrpg.db" || c.StaticDir != "client/static" || c.Debug {
		t.Fatalf("defaults: %+v %v", c, err)
	}

	t.Setenv("PORT", "9000")
	t.Setenv("DB_PATH", "/tmp/x.db")
	t.Setenv("DEBUG", "1")
	c, _ = FromEnv()
	if c.Port != 9000 || c.DBPath != "/tmp/x.db" || !c.Debug {
		t.Fatalf("overrides: %+v", c)
	}

	t.Setenv("PORT", "eighty")
	if _, err := FromEnv(); err == nil {
		t.Fatal("non-numeric PORT should be an error")
	}
}

{ pkgs, lib, config, ... }:

{
  # ── Languages ────────────────────────────────────────────────────────────────

  languages.go = {
    enable = true;
    package = pkgs.go_1_24;
  };

  languages.javascript = {
    enable = true;
    package = pkgs.nodejs_24;
  };

  languages.python = {
    enable = true;
    package = pkgs.python312;
  };

  # ── Packages ──────────────────────────────────────────────────────────────────

  packages = with pkgs; [
    golangci-lint
    go-mockery
    uv
    jujutsu
    jq
    gh
    curl
    gnumake
  ];

  # ── PostgreSQL service (replaces Docker) ──────────────────────────────────────

  services.postgres = {
    enable = true;
    package = pkgs.postgresql_16;
    listen_addresses = "127.0.0.1";
    port = 5432;
    initialDatabases = [{ name = "mattermost_test"; }];
    initialScript = pkgs.writeText "pg-init.sql" ''
      DO $$
      BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'mmuser') THEN
          CREATE USER mmuser WITH PASSWORD 'mostest' CREATEDB;
        END IF;
      END
      $$;
      GRANT ALL PRIVILEGES ON DATABASE mattermost_test TO mmuser;
    '';
  };

  # ── Environment variables ──────────────────────────────────────────────────────

  env = {
    MM_DEV                        = "true";
    OPENCLAW_GATEWAY_URL          = "http://127.0.0.1:18789";
    MM_FEATUREFLAGS_ENABLEAIAGENTS = "true";
    # Matches the default in server/public/model/config.go
    MM_SQLSETTINGS_DATASOURCE     = "postgres://mmuser:mostest@127.0.0.1/mattermost_test?sslmode=disable&connect_timeout=10&binary_parameters=yes";
    GOPATH                        = "$HOME/.go";
  };

  # ── Processes (devenv up starts all three) ────────────────────────────────────
  # Run `devenv up` to start postgres + server + webapp together.
  # Run `devenv services up` to start postgres only, then start server/webapp manually.

  processes = {
    server = {
      exec = "cd ${config.devenv.root}/server && go run ./cmd/mattermost";
      process-compose.depends_on.postgres.condition = "process_healthy";
    };
    webapp = {
      exec = "cd ${config.devenv.root}/webapp && npm run dev-server --workspace=channels";
    };
  };

  # ── Shell hook ─────────────────────────────────────────────────────────────────

  enterShell = ''
    export PATH="$GOPATH/bin:$PATH"
    # Auto-load .env.agents if present (OpenClaw token, admin credentials, etc.)
    if [ -f "${config.devenv.root}/.env.agents" ]; then
      set -a; source "${config.devenv.root}/.env.agents"; set +a
    fi
    echo ""
    echo "Mattermost Agent Platform — Nix dev environment"
    echo ""
    echo "  devenv up               start PostgreSQL + server + webapp"
    echo "  devenv services up      start PostgreSQL only"
    echo "  mise run agents:provision   provision demo agents (server must be up)"
    echo "  mise run agents:status      list provisioned agents"
    echo "  mise run docs:serve         live-preview docs at http://localhost:8000"
    echo ""
  '';
}

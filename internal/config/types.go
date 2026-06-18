package config

type Rest struct {
	Version       string              `yaml:"version"`
	Module        string              `yaml:"module"`
	ProjectPath   string              `yaml:"project_path"`
	Language      string              `yaml:"language"`
	GoVersion     string              `yaml:"go_version"`
	Environment   string              `yaml:"environment"`
	HTTP          HTTP                `yaml:"http"`
	SQL           Enabled             `yaml:"sql"`
	AutoSQLC      Enabled             `yaml:"auto_sqlc"`
	Mongo         Enabled             `yaml:"mongo"`
	Auth          Enabled             `yaml:"auth"`
	SafeReload    Enabled             `yaml:"safe_reload"`
	Logging       Logging             `yaml:"logging"`
	OpenAPI       OpenAPI             `yaml:"openapi"`
	Docker        Docker              `yaml:"docker"`
	Testing       Testing             `yaml:"testing"`
	Observability Observability       `yaml:"observability"`
	Features      ApplicationFeatures `yaml:"features"`
}

type HTTP struct {
	Framework        string          `yaml:"framework"`
	Host             string          `yaml:"host"`
	Port             int             `yaml:"port"`
	BasePath         string          `yaml:"base_path"`
	Timeouts         HTTPTimeouts    `yaml:"timeouts"`
	Limits           HTTPLimits      `yaml:"limits"`
	DatabasePool     DatabasePool    `yaml:"database_pool"`
	GracefulShutdown GeneratedSwitch `yaml:"graceful_shutdown"`
	Health           Health          `yaml:"health"`
	Middleware       Middleware      `yaml:"middleware"`
}

type HTTPTimeouts struct {
	ReadHeader string `yaml:"read_header"`
	Read       string `yaml:"read"`
	Write      string `yaml:"write"`
	Idle       string `yaml:"idle"`
	Shutdown   string `yaml:"shutdown"`
}

type HTTPLimits struct {
	MaxBodyBytes int64 `yaml:"max_body_bytes"`
}
type DatabasePool struct {
	Enabled         Enabled `yaml:"enabled"`
	MaxOpenConns    int     `yaml:"max_open_conns"`
	MaxIdleConns    int     `yaml:"max_idle_conns"`
	ConnMaxIdleTime string  `yaml:"conn_max_idle_time"`
	ConnMaxLifetime string  `yaml:"conn_max_lifetime"`
	PingTimeout     string  `yaml:"ping_timeout"`
}
type GeneratedSwitch struct {
	Enabled Enabled `yaml:"enabled"`
}
type Health struct {
	Enabled Enabled `yaml:"enabled"`
	Path    string  `yaml:"path"`
}
type Middleware struct {
	RequestID RequestID `yaml:"request_id"`
	CORS      CORS      `yaml:"cors"`
	Recovery  Recovery  `yaml:"recovery"`
}
type RequestID struct {
	Enabled Enabled `yaml:"enabled"`
	Header  string  `yaml:"header"`
}
type Recovery struct {
	Enabled       Enabled `yaml:"enabled"`
	ExposeDetails bool    `yaml:"expose_details"`
}
type CORS struct {
	Enabled          Enabled  `yaml:"enabled"`
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	ExposeHeaders    []string `yaml:"expose_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           string   `yaml:"max_age"`
}

type Logging struct {
	Enabled  Enabled           `yaml:"enabled"`
	Library  string            `yaml:"library"`
	Level    string            `yaml:"level"`
	Format   string            `yaml:"format"`
	Output   LoggingOutput     `yaml:"output"`
	Rotation LogRotation       `yaml:"rotation"`
	Fields   map[string]string `yaml:"fields"`
	Redact   []string          `yaml:"redact"`
}
type LoggingOutput struct {
	Type string `yaml:"type"`
	File string `yaml:"file"`
}
type LogRotation struct {
	Enabled    Enabled `yaml:"enabled"`
	MaxSizeMB  int     `yaml:"max_size_mb"`
	MaxBackups int     `yaml:"max_backups"`
	MaxAgeDays int     `yaml:"max_age_days"`
	Compress   bool    `yaml:"compress"`
}

type OpenAPI struct {
	Enabled         Enabled `yaml:"enabled"`
	Output          string  `yaml:"output"`
	Title           string  `yaml:"title"`
	Version         string  `yaml:"version"`
	Description     string  `yaml:"description"`
	ServerURL       string  `yaml:"server_url"`
	WithUI          Enabled `yaml:"with_ui"`
	UIPath          string  `yaml:"ui_path"`
	SpecPath        string  `yaml:"spec_path"`
	SecuritySchemes string  `yaml:"security_schemes"`
}

type Docker struct {
	Enabled            Enabled           `yaml:"enabled"`
	Output             string            `yaml:"output"`
	DockerignoreOutput string            `yaml:"dockerignore_output"`
	BuildImage         string            `yaml:"build_image"`
	RuntimeImage       string            `yaml:"runtime_image"`
	Binary             string            `yaml:"binary"`
	Port               int               `yaml:"port"`
	User               string            `yaml:"user"`
	CGOEnabled         bool              `yaml:"cgo_enabled"`
	Healthcheck        DockerHealthcheck `yaml:"healthcheck"`
}
type DockerHealthcheck struct {
	Enabled  Enabled `yaml:"enabled"`
	Path     string  `yaml:"path"`
	Interval string  `yaml:"interval"`
	Timeout  string  `yaml:"timeout"`
	Retries  int     `yaml:"retries"`
}

type Testing struct {
	HandlerTests Enabled `yaml:"handler_tests"`
	Curl         Enabled `yaml:"curl"`
}
type Observability struct {
	Metrics Metrics `yaml:"metrics"`
}
type Metrics struct {
	Enabled   Enabled          `yaml:"enabled"`
	Provider  string           `yaml:"provider"`
	Path      string           `yaml:"path"`
	Namespace string           `yaml:"namespace"`
	Collect   MetricCollection `yaml:"collect"`
	Labels    []string         `yaml:"labels"`
}
type MetricCollection struct {
	HTTPRequests     bool `yaml:"http_requests"`
	RequestDuration  bool `yaml:"request_duration"`
	ResponseSize     bool `yaml:"response_size"`
	InFlightRequests bool `yaml:"in_flight_requests"`
	DatabasePool     bool `yaml:"database_pool"`
}

type ApplicationFeatures struct {
	Makefile  GeneratedFile    `yaml:"makefile"`
	Gitignore GitignoreFeature `yaml:"gitignore"`
	Env       EnvFeature       `yaml:"env"`
	InitDB    GeneratedFile    `yaml:"init_db"`
	CI        GeneratedFile    `yaml:"ci"`
	CD        GeneratedFile    `yaml:"cd"`
}
type GeneratedFile struct {
	Enabled Enabled `yaml:"enabled"`
	Output  string  `yaml:"output"`
}
type GitignoreFeature struct {
	Enabled Enabled `yaml:"enabled"`
	Output  string  `yaml:"output"`
	Append  bool    `yaml:"append"`
}
type EnvFeature struct {
	Enabled          Enabled `yaml:"enabled"`
	Output           string  `yaml:"output"`
	GenerateLocalEnv bool    `yaml:"generate_local_env"`
}

type SQL struct {
	Version         string       `yaml:"version"`
	Database        string       `yaml:"database"`
	Connection      DBConnection `yaml:"db_connection"`
	ORM             Enabled      `yaml:"orm"`
	Engine          string       `yaml:"engine"`
	SQLC            SQLC         `yaml:"sqlc"`
	InitMigration   Enabled      `yaml:"init_migration"`
	MigrationEngine string       `yaml:"migration_engine"`
	MigrationOutput string       `yaml:"migration_output"`
}
type SQLC struct {
	Enabled Enabled `yaml:"enable"`
	Example Enabled `yaml:"sqlc_example"`
	Path    string  `yaml:"sqlc_path"`
}
type DBConnection struct {
	DBName             string  `yaml:"db_name"`
	UserName           string  `yaml:"user_name"`
	UserPassword       string  `yaml:"user_password"`
	LegacyUserPassword string  `yaml:"usere_password"`
	PoolConnection     Enabled `yaml:"pool_connection"`
	Options            string  `yaml:"options"`
}
type Bundle struct {
	Dir  string
	Rest Rest
	SQL  *SQL
}
